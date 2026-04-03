package networking

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"github.com/kiryuhakipyatok/aloh-networking/internal/client"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/models"
	"github.com/kiryuhakipyatok/aloh-networking/internal/utils"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
)

func (ns *networkingServ) processData(id string, data []byte) {
	op := "networkingServ.processData"
	log := ns.logger.AddOp(op)
	msgType := data[0]
	payload := make([]byte, len(data)-1)
	copy(payload, data[1:])
	switch msgType {
	case CHAT:
		chatHdlr, ok := ns.onChatHandler.Load().(handler)
		if ok {
			chatHdlr(id, payload)
		}
	case VOICE:
		voiceHdlr, ok := ns.onVoiceHandler.Load().(handler)
		if ok {
			voiceHdlr(id, payload)
		}
	case VIDEO:
		videoHdlr, ok := ns.onVideoHandler.Load().(handler)
		if ok {
			videoHdlr(id, payload)
		}
	default:
		err := errors.New("invalid type")
		log.Error("failed to process data", logger.Err(err), logger.Attr("msgType", msgType))
	}
}

func (ns *networkingServ) disconnectSession(session *models.Session) {
	session.Closing.Do(func() {
		op := "networkingServ.disconnectSession"
		log := ns.logger.AddOp(op)
		userIdLog := logger.Attr("userId", session.UserID)
		log.Info("user disconnecting...", userIdLog)
		if session.Agent != nil {
			if err := session.Agent.Close(); err != nil {
				log.Error("failed to close ice agent", logger.Err(err), userIdLog)
			}
		}
		if session.Conn != nil {
			if err := session.Conn.CloseWithError(0, "disconnected"); err != nil {
				log.Error("failed to close quic conn", logger.Err(err), userIdLog)
			}
		}
		if err:=ns.signalingClient.DeleteFromSession(context.Background(), session.UserID);err!=nil{
			log.Error("failed to delete from session", logger.Err(err), userIdLog)
		}
		if err := ns.sessionRepo.Delete(context.Background(), session.UserID); err != nil {
			log.Error("failed to delete session", logger.Err(err), userIdLog)
		} else {
			log.Info("user disconnected", userIdLog)
		}
	})
}

func (ns *networkingServ) createSession(ctx context.Context, rid string, isInitiator bool) (*models.Session, error) {
	op := "networkingServ.createSession"
	log := ns.logger.AddOp(op)
	ridLog := logger.Attr("receiverId", rid)
	select {
	case <-ns.closeCtx.Done():
		return nil, errs.AppClosing(op)
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
	}
	log.Info("creating new session...", ridLog)
	if err := ns.signalingClient.AddInSession(ctx, rid); err != nil {
		log.Error("failed to add session", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}

	username, password, err := ns.signalingClient.GetCreds(ctx)
	if err != nil {
		log.Error("failed to fetch creds", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	agent, err := ice.NewAgent(&ice.AgentConfig{
		Urls: []*stun.URI{
			{Scheme: stun.SchemeTypeSTUN, Host: ns.cfg.STUNHost, Port: ns.cfg.STUNPort, Proto: stun.ProtoTypeUDP},
			{Scheme: stun.SchemeTypeTURN, Host: ns.cfg.TURNHost, Port: ns.cfg.TURNPort, Username: username, Password: password, Proto: stun.ProtoTypeUDP},
			{Scheme: stun.SchemeTypeTURN, Host: ns.cfg.TURNHost, Port: ns.cfg.TURNPort, Username: username, Password: password, Proto: stun.ProtoTypeTCP},
		},
		NetworkTypes: []ice.NetworkType{
			ice.NetworkTypeUDP4,
			ice.NetworkTypeUDP6,
			ice.NetworkTypeTCP4,
			ice.NetworkTypeTCP6,
		},
	})
	if err != nil {
		log.Error("failed to create agent", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	session := &models.Session{
		UserID:      rid,
		Agent:       agent,
		IsInitiator: isInitiator,
		CredsChan:   make(chan struct{}, 1),
		Closing:     sync.Once{},
	}

	localFrag, localPwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Error("failed to get local user credentials", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	creds := fmt.Appendf(nil, "%s %s", localFrag, localPwd)
	sdp := utils.SetFirstByte(CREDS, creds)
	log.Debug("sdp (credentials) creating", ridLog)
	if err := ns.signalingClient.NewSDP(ctx, sdp, []string{session.UserID}); err != nil {
		log.Error("failed to create new sdp (credentials)", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}

	agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}
		go func(c ice.Candidate) {
			sdpCtx, cancel := context.WithTimeout(context.Background(), ns.cfg.NewSDPTimeout)
			defer cancel()
			candidate := []byte(c.Marshal())
			sdp := utils.SetFirstByte(CANDIDATE, candidate)
			log.Debug("sdp (candidate) creating", ridLog)
			if err := ns.signalingClient.NewSDP(sdpCtx, sdp, []string{rid}); err != nil {
				log.Error("failed to create new sdp (candidate)", logger.Err(err), ridLog)
			}
		}(c)

	})
	if err = agent.OnConnectionStateChange(func(c ice.ConnectionState) {
		if c.String() == CLOSED || c.String() == DISCONNECTED || c.String() == FAILED {
			ns.disconnectSession(session)
		}
	}); err != nil {
		log.Error("failed on connection state change", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	go func() {
		log.Info("candidate gathering...", ridLog)
		if err := agent.GatherCandidates(); err != nil {
			log.Error("failed to gather candidates", logger.Err(err), ridLog)
			ns.disconnectSession(session)
		}
	}()
	log.Debug("session saving...", ridLog)
	if err := ns.sessionRepo.Add(ctx, rid, session); err != nil {
		log.Error("failed to save session", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	return session, nil
}

func (ns *networkingServ) receiveConnects() error {
	op := "networkingServ.receiveConnect"
	log := ns.logger.AddOp(op)
	log.Info("connection receiving...")

	for sdp := range ns.receiveSDPs {
		select {
		case <-ns.closeCtx.Done():
			return errs.AppClosing(op)
		default:
		}
		go func(sdp client.ReplyMessage) {
			if len(sdp.Payload) < 2 {
				return
			}
			ctx, cancel := context.WithTimeout(ns.closeCtx, time.Second*5)
			defer cancel()
			senderId := sdp.Sender
			senderIdLog := logger.Attr("senderId", senderId)
			log.Debug("received new sdp", senderIdLog)
			v, err, _ := ns.sdpsGroup.Do(sdp.Sender, func() (any, error) {
				return ns.getSession(ctx, senderId)
			})
			if err != nil {
				log.Error("failed to get session", logger.Err(err), senderIdLog)
				return
			}
			session, ok := v.(*models.Session)
			if !ok {
				log.Error("invalid session type", senderIdLog)
				return
			}
			switch sdp.Payload[0] {
			case CREDS:
				log.Debug("credentials processing", senderIdLog)
				creds := strings.Split(string(sdp.Payload[1:]), " ")
				if len(creds) < 2 {
					return
				}
				remoteUrfrag := creds[0]
				remotePwd := creds[1]
				if err := session.Agent.SetRemoteCredentials(remoteUrfrag, remotePwd); err != nil {
					log.Error("failed to set remote credential", logger.Err(err), senderIdLog)
					return
				}
				select {
				case session.CredsChan <- struct{}{}:
				default:
				}
				log.Debug("credentials processed", senderIdLog)

			case CANDIDATE:
				log.Debug("candidate processing", senderIdLog)
				c, err := ice.UnmarshalCandidate(string(sdp.Payload[1:]))
				if err != nil {
					log.Error("failed to unmarshal candidate", logger.Err(err), senderIdLog)
					return
				}
				if err := session.Agent.AddRemoteCandidate(c); err != nil {
					log.Error("failed to add remote candidate", logger.Err(err), senderIdLog)
					return
				}
				log.Debug("candidate processed", senderIdLog)
			}
		}(sdp)
	}

	return nil

}

func (ns *networkingServ) getSession(ctx context.Context, id string) (*models.Session, error) {
	op := "networkingServ.getSession"
	log := ns.logger.AddOp(op)
	userLog := logger.Attr("userId", id)
	select {
	case <-ns.closeCtx.Done():
		return nil, errs.AppClosing(op)
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
	}
	log.Info("session getting...", userLog)
	session, err := ns.sessionRepo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, errs.ErrNotFoundBase) {
			session, err = ns.createSession(ctx, id, false)
			if err != nil {
				log.Error("failed to create session", logger.Err(err), userLog)
				return nil, errs.NewAppError(op, err)
			}

			go func() {
				estCtx, cancel := context.WithTimeout(context.Background(), ns.cfg.EstablishConnTimeout)
				defer cancel()
				if err := ns.establishConnection(estCtx, session); err != nil {
					log.Error("failed to establish connection", logger.Err(err), userLog)
					ns.disconnectSession(session)
				}
			}()
		} else {
			log.Error("failed to get session", logger.Err(err), userLog)
			return nil, errs.NewAppError(op, err)
		}
	}

	return session, nil
}

func (ns *networkingServ) establishConnection(ctx context.Context, session *models.Session) error {
	op := "networkingServ.establishConnection"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", session.UserID)
	select {
	case <-session.CredsChan:
	case <-ns.closeCtx.Done():
		return errs.AppClosing(op)
	}
	log.Info("connection establishing...", userIdLog)

	var (
		conn     *ice.Conn
		quicConn *quic.Conn
	)
	remoteUfrag, remotePwd, err := session.Agent.GetRemoteUserCredentials()
	if err != nil {
		log.Error("failed to get remote user credentials", logger.Err(err), userIdLog)
		return errs.NewAppError(op, err)
	}
	quicConf := &quic.Config{
		KeepAlivePeriod:      ns.cfg.KeepAlivePeriodTimeout,
		MaxIdleTimeout:       ns.cfg.IdleTimeout,
		HandshakeIdleTimeout: ns.cfg.HandshakeTimeout,
		EnableDatagrams:      true,
	}
	switch session.IsInitiator {
	case true:
		log.Debug("dialing agent connection", userIdLog)
		conn, err = session.Agent.Dial(ctx, remoteUfrag, remotePwd)
		if err != nil {
			fmt.Println(ctx.Err())
			log.Error("failed to dial agent", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}

		remoteAddr := conn.RemoteAddr()
		packetConn := &PacketConnWrapper{
			Conn:      conn,
			FixedAddr: remoteAddr,
		}

		t := quic.Transport{
			Conn: packetConn,
		}
		tlsConf := &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         ns.cfg.NextProtos,
		}
		quicConn, err = t.Dial(ctx, remoteAddr, tlsConf, quicConf)
		if err != nil {
			log.Error("failed to dial quic conn", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}

	case false:
		log.Info("accepting agent connection", userIdLog)
		conn, err = session.Agent.Accept(ctx, remoteUfrag, remotePwd)
		if err != nil {
			log.Error("failed to accept agent connection", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
		remoteAddr := conn.RemoteAddr()
		packetConn := &PacketConnWrapper{
			Conn:      conn,
			FixedAddr: remoteAddr,
		}
		t := quic.Transport{
			Conn: packetConn,
		}
		listener, err := t.Listen(ns.tlsConf, quicConf)
		if err != nil {
			log.Error("failed to start listen quic connections", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
		defer func() {
			if err := listener.Close(); err != nil {
				log.Error("failed to close quic listener", logger.Err(err), userIdLog)
			}
		}()
		quicConn, err = listener.Accept(ctx)
		if err != nil {
			log.Error("failed to accept quic connection", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
	}
	session.Conn = quicConn
	log.Info("connection established successfully", userIdLog)
	go ns.handleConnection(session)
	return nil

}

func (ns *networkingServ) handleConnection(session *models.Session) {
	op := "networkingServ.handleConnection"
	log := ns.logger.AddOp(op)
	remoteAddrLog := logger.Attr("remoteAddr", session.Conn.RemoteAddr())
	localAddrLog := logger.Attr("localAddr", session.Conn.LocalAddr())
	connLog := logger.NewLogData(remoteAddrLog, localAddrLog)
	log.Info("connection handling...", connLog...)
	defer func() {
		ns.disconnectSession(session)
		log.Info("connection handling stopped")
	}()
	go ns.receiveDatagrams(session)
	ns.receiveStreams(session)
}

func (ns *networkingServ) receiveStreams(session *models.Session) {
	op := "networkingServ.receiveStreams"
	log := ns.logger.AddOp(op)
	remoteAddrLog := logger.Attr("remoteAddr", session.Conn.RemoteAddr())
	localAddrLog := logger.Attr("localAddr", session.Conn.LocalAddr())
	connLog := logger.NewLogData(remoteAddrLog, localAddrLog)
	log.Info("streams receiving...", connLog...)
	for {
		stream, err := session.Conn.AcceptUniStream(ns.closeCtx)
		if err != nil {
			if cerr := utils.CheckErr(ns.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to accept uni stream", logger.Err(err), remoteAddrLog, localAddrLog)
			return
		}

		data, err := io.ReadAll(stream)
		if err != nil {
			log.Error("failed to read data from stream", logger.Err(err), remoteAddrLog, localAddrLog)
			return
		}
		if len(data) < 1 {
			log.Error("received empty data", remoteAddrLog, localAddrLog)
			continue
		}
		msgLog := logger.Attr("msg", string(data[1:]))
		msgLenLog := logger.Attr("msgLen", len(data[1:]))
		log.Info("new msg from stream received", remoteAddrLog, localAddrLog, msgLog, msgLenLog)
		stream.CancelRead(0)
		ns.processData(session.UserID, data)
	}
}

func (ns *networkingServ) receiveDatagrams(session *models.Session) {
	op := "networkingServ.receiveDatagrams"
	log := ns.logger.AddOp(op)
	sparseLog := log.Sparse(ns.cfg.DatagramLogTargetCount)
	remoteAddrLog := logger.Attr("remoteAddr", session.Conn.RemoteAddr())
	localAddrLog := logger.Attr("localAddr", session.Conn.LocalAddr())
	connLog := logger.NewLogData(remoteAddrLog, localAddrLog)
	sparseLog.Info(ns.receiveDatagramLogCount, "datagram receiving...", connLog...)
	ns.receiveDatagramLogCount = 0
	for {
		ns.receiveDatagramLogCount++
		data, err := session.Conn.ReceiveDatagram(ns.closeCtx)
		if err != nil {
			if cerr := utils.CheckErr(ns.closeCtx, err); cerr == nil {
				ns.disconnectSession(session)
				return
			}
			sparseLog.Error(ns.receiveDatagramLogCount, "failed to receive datagram", logger.Err(err), remoteAddrLog, localAddrLog)
			return
		}
		if len(data) < 1 {
			sparseLog.Error(ns.receiveDatagramLogCount, "received empty data", remoteAddrLog, localAddrLog)
			continue
		}
		msgLog := logger.Attr("msgLen", len(data[1:]))
		sparseLog.Info(ns.receiveDatagramLogCount, "new datagram received", remoteAddrLog, localAddrLog, msgLog)
		ns.processData(session.UserID, data)
	}
}

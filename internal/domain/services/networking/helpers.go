package networking

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/google/uuid"

	"github.com/kiryuhakipyatok/aloh-networking/internal/client"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/e2ee"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/models"
	"github.com/kiryuhakipyatok/aloh-networking/internal/utils"
	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"

	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
)

func (ns *networkingServ) processData(id uuid.UUID, data []byte) {
	op := "networkingServ.processData"
	log := ns.logger.AddOp(op)
	msgType := data[0]
	payload := make([]byte, len(data)-1)
	copy(payload, data[1:])
	switch msgType {
	case CHAT:
		chatHdlr, ok := ns.onChatHandler.Load().(dataHandler)
		if ok {
			chatHdlr(id, payload)
		}
	case VOICE:
		voiceHdlr, ok := ns.onVoiceHandler.Load().(dataHandler)
		if ok {
			voiceHdlr(id, payload)
		}
	case VIDEO:
		videoHdlr, ok := ns.onVideoHandler.Load().(dataHandler)
		if ok {
			videoHdlr(id, payload)
		}
	default:
		err := errors.New("invalid type")
		log.Error("failed to process data", logger.Err(err), logger.Attr("msgType", msgType))
	}
}

func (ns *networkingServ) disconnectSession(session *models.Session, isLeaveInitiator bool) {
	session.Closing.Do(func() {
		op := "networkingServ.disconnectSession"
		log := ns.logger.AddOp(op)
		userIdLog := logger.Attr("userId", session.UserID)
		log.Info("user disconnecting...", userIdLog)
		if session.EventStream != nil {
			if err := session.EventStream.Close(); err != nil {
				log.Error("failed to close event stream", logger.Err(err), userIdLog)
			} else {
				select {
				case <-session.EventStream.Context().Done():
				case <-ns.closeCtx.Done():
				}
			}
		}

		if session.Conn != nil {
			if err := session.Conn.CloseWithError(0, "disconnected"); err != nil {
				log.Error("failed to close quic conn", logger.Err(err), userIdLog)
			}
		}
		if session.Agent != nil {
			if err := session.Agent.GracefulClose(); err != nil {
				log.Error("failed to close ice agent", logger.Err(err), userIdLog)
			}
		}

		if err := ns.sessionRepo.Delete(context.Background(), session.UserID, session); err != nil {
			log.Error("failed to delete session", logger.Err(err), userIdLog)
		}

		if err := ns.signalingClient.DeleteFromSession(context.Background(), session.UserID); err != nil {
			log.Error("failed to delete from session", logger.Err(err), userIdLog)
		}
		if !isLeaveInitiator {
			disconnHdlr, ok := ns.onPeerDisconnectedHandler.Load().(connectionHandler)
			if ok {
				disconnHdlr(session.UserID)
			}
		}

		log.Info("user disconnected", userIdLog)

	})
}

func (ns *networkingServ) resetSession(session *models.Session) {
	op := "networkingServ.resetSession"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", session.UserID)
	log.Info("session reseting...", userIdLog)
	if session.Conn != nil {
		if err := session.Conn.CloseWithError(0, "disconnected"); err != nil {
			log.Error("failed to close quic conn", logger.Err(err), userIdLog)
		}
	}
	if session.Agent != nil {
		if err := session.Agent.GracefulClose(); err != nil {
			log.Error("failed to close ice agent", logger.Err(err), userIdLog)
		}
	}

	if err := ns.sessionRepo.Delete(context.Background(), session.UserID, session); err != nil {
		log.Error("failed to delete session", logger.Err(err), userIdLog)
	}

	log.Info("session reseted", userIdLog)
}

func (ns *networkingServ) createSession(ctx context.Context, rid uuid.UUID, isInitiator bool) (*models.Session, error) {
	op := "networkingServ.createSession"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	ridLog := logger.Attr("receiverId", rid)
	select {
	case <-ns.closeCtx.Done():
		return nil, errs.AppClosing(op)
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
	}
	log.Info("creating new session...", ridLog, userIdLog)
	if err := ns.signalingClient.AddInSession(ctx, rid); err != nil {
		log.Error("failed to add session", logger.Err(err), ridLog, userIdLog)
		return nil, errs.NewAppError(op, err)
	}
	username, password, err := ns.signalingClient.GetCreds(ctx)
	if err != nil {
		log.Error("failed to fetch creds", logger.Err(err), ridLog, userIdLog)
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
		DisconnectedTimeout: &ns.cfg.DisconnectedTimeout,
	})
	if err != nil {
		log.Error("failed to create agent", logger.Err(err), ridLog, userIdLog)
		return nil, errs.NewAppError(op, err)
	}
	session := &models.Session{
		UserID:      rid,
		Agent:       agent,
		IsInitiator: isInitiator,
		CredsChan:   make(chan struct{}, 1),
		Closing:     sync.Once{},
		ReadyChan:   make(chan struct{}, 1),
	}

	localFrag, localPwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Error("failed to get local user credentials", logger.Err(err), ridLog, userIdLog)
		return nil, errs.NewAppError(op, err)
	}
	creds := fmt.Appendf(nil, "%s %s", localFrag, localPwd)
	sdp := utils.SetFirstByte(CREDS, creds)
	log.Debug("sdp (credentials) creating", ridLog, userIdLog)
	if err := ns.signalingClient.NewSDP(ctx, sdp, []uuid.UUID{session.UserID}); err != nil {
		log.Error("failed to create new sdp (credentials)", logger.Err(err), ridLog, userIdLog)
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
			log.Debug("sdp (candidate) creating", ridLog, userIdLog)
			if err := ns.signalingClient.NewSDP(sdpCtx, sdp, []uuid.UUID{rid}); err != nil {
				log.Error("failed to create new sdp (candidate)", logger.Err(err), ridLog, userIdLog)
			}
		}(c)

	})
	if err = agent.OnConnectionStateChange(func(c ice.ConnectionState) {
		if c == FAILED || c == DISCONNECTED {
			log.Info("ice connection failed, reconnect", ridLog)
			ns.resetSession(session)
			if session.IsInitiator {
				go func() {
					b := backoff.NewExponentialBackOff()
					b.InitialInterval = 1 * time.Second
					for {
						time.Sleep(b.NextBackOff())
						if ns.closeCtx.Err() != nil {
							log.Info("app is closing, stop ice reconnecting", ridLog)
							return
						}

						log.Info("reconnecting...", ridLog)
						newSession, err := ns.createAndEstablish(context.Background(), rid, INITIATOR)
						if err != nil {
							log.Error("failed to reconnect", logger.Err(err), ridLog)
							continue
						}

						<-newSession.ReadyChan

						if newSession.Conn == nil {
							log.Error("establish connection failed, retrying...", ridLog)
							continue
						}
						log.Info("reconnected successfully", ridLog)
						return
					}

				}()
			}
		}
	}); err != nil {
		log.Error("failed on connection state change", logger.Err(err), ridLog, userIdLog)
		return nil, errs.NewAppError(op, err)
	}
	go func() {
		log.Info("candidate gathering...", ridLog, userIdLog)
		if err := agent.GatherCandidates(); err != nil {
			log.Error("failed to gather candidates", logger.Err(err), ridLog, userIdLog)
			//ns.disconnectSession(session)
			return
		}
	}()
	log.Debug("session saving...", ridLog, userIdLog)
	if err := ns.sessionRepo.Add(ctx, rid, session); err != nil {
		log.Error("failed to save session", logger.Err(err), ridLog, userIdLog)
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
			v, err, _ := ns.sdpsGroup.Do(sdp.Sender.String(), func() (any, error) {
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

				if session.Conn != nil {
					log.Info("reconnecting", senderIdLog)
					ns.resetSession(session)
					createCtx, cancel := context.WithTimeout(ns.closeCtx, time.Second*5)
					defer cancel()
					var err error
					session, err = ns.createAndEstablish(createCtx, senderId, NOT_INITIATOR)
					if err != nil {
						log.Error("failed to recreate session", logger.Err(err), senderIdLog)
						return
					}
				}
				if err := session.Agent.SetRemoteCredentials(remoteUrfrag, remotePwd); err != nil {
					log.Error("failed to set remote credential", logger.Err(err), senderIdLog)
					return
				}
				select {
				case session.CredsChan <- struct{}{}:
				default:
				}
				// }

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

func (ns *networkingServ) createAndEstablish(ctx context.Context, id uuid.UUID, isInitiator bool) (*models.Session, error) {
	op := "networkingServ.createAndEstablish"
	log := ns.logger.AddOp(op)
	userLog := logger.Attr("userId", id)
	session, err := ns.createSession(ctx, id, isInitiator)
	if err != nil {
		log.Error("failed to create session", logger.Err(err), userLog)
		return nil, errs.NewAppError(op, err)
	}

	go func() {
		estCtx, cancel := context.WithTimeout(context.Background(), ns.cfg.EstablishConnTimeout)
		defer cancel()
		if err := ns.establishConnection(estCtx, session); err != nil {
			log.Error("failed to establish connection", logger.Err(err), userLog)
			close(session.ReadyChan)
			//ns.disconnectSession(session)
			ns.resetSession(session)
		}
	}()

	return session, nil
}

func (ns *networkingServ) getSession(ctx context.Context, id uuid.UUID) (*models.Session, error) {
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
			session, err = ns.createAndEstablish(ctx, id, NOT_INITIATOR)
			if err != nil {
				log.Error("failed to create session and establish connection", logger.Err(err), userLog)
				return nil, errs.NewAppError(op, err)
			}
		} else {
			log.Error("failed to get session", logger.Err(err), userLog)
			return nil, errs.NewAppError(op, err)
		}
	}

	// if session.Conn != nil {
	// 	log.Info("sessions already exists, reconnecting", userLog)
	// 	ns.disconnectSession(session)
	// 	session, err = ns.createAndEstablish(ctx, id, NOT_INITIATOR)
	// 	if err != nil {
	// 		log.Error("failed to create and establish connection", logger.Err(err), userLog)
	// 		return nil, errs.NewAppError(op, err)
	// 	}
	// }

	return session, nil
}

func (ns *networkingServ) establishConnection(ctx context.Context, session *models.Session) error {
	op := "networkingServ.establishConnection"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", session.UserID)
	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
	select {
	case <-session.CredsChan:
	case <-ns.closeCtx.Done():
		return errs.AppClosing(op)
	}
	log.Info("connection establishing...", idsLog)

	var (
		conn     *ice.Conn
		quicConn *quic.Conn
	)
	remoteUfrag, remotePwd, err := session.Agent.GetRemoteUserCredentials()
	if err != nil {
		log.Error("failed to get remote user credentials", logger.Err(err), userIdLog, receiverIdLog)
		return errs.NewAppError(op, err)
	}
	quicConf := &quic.Config{
		KeepAlivePeriod:      ns.cfg.KeepAlivePeriodTimeout,
		MaxIdleTimeout:       ns.cfg.MaxIdleTimeout,
		HandshakeIdleTimeout: ns.cfg.HandshakeTimeout,
		EnableDatagrams:      true,
	}
	switch session.IsInitiator {
	case INITIATOR:
		log.Debug("dialing agent connection", idsLog...)
		conn, err = session.Agent.Dial(ctx, remoteUfrag, remotePwd)
		if err != nil {
			log.Error("failed to dial agent", logger.Err(err), userIdLog, receiverIdLog)
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
			log.Error("failed to dial quic conn", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}

	case NOT_INITIATOR:
		log.Info("accepting agent connection", idsLog...)
		conn, err = session.Agent.Accept(ctx, remoteUfrag, remotePwd)
		if err != nil {
			log.Error("failed to accept agent connection", logger.Err(err), userIdLog, receiverIdLog)
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
			log.Error("failed to start listen quic connections", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
		defer func() {
			if err := listener.Close(); err != nil {
				log.Error("failed to close quic listener", logger.Err(err), userIdLog, receiverIdLog)
			}
		}()
		quicConn, err = listener.Accept(ctx)
		if err != nil {
			log.Error("failed to accept quic connection", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
	}
	session.Conn = quicConn
	if err := setupE2EE(ctx, session); err != nil {
		log.Error("failed to setup e2ee", logger.Err(err), receiverIdLog, userIdLog)
		return errs.NewAppError(op, err)
	}

	if err := ns.initEventStream(ctx, session); err != nil {
		log.Error("failed to init event stream", logger.Err(err), receiverIdLog, userIdLog)
		return errs.NewAppError(op, err)
	}

	//if !session.IsInitiator {
	connHdlr, ok := ns.onPeerConnectedHandler.Load().(connectionHandler)
	if ok {
		connHdlr(session.UserID)
	}
	//}

	log.Info("e2ee connection established successfully", idsLog...)

	go ns.handleConnection(session)
	close(session.ReadyChan)
	return nil

}

// func (ns *networkingServ) reconnect(ctx context.Context, session *models.Session, remoteUfrag, remotePwd string) error {
// 	op := "networking.reconnect"
// 	log := ns.logger.AddOp(op)
// 	userIdLog := logger.Attr("userId", ns.id)
// 	receiverIdLog := logger.Attr("receiverId", session.UserID)
// 	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
// 	log.Info("reconnecting...", idsLog...)
// 	if err := session.Agent.Restart(remoteUfrag, remotePwd); err != nil {
// 		log.Error("failed to restart agent", logger.Err(err), userIdLog, receiverIdLog)
// 		return errs.NewAppError(op, err)
// 	}
// 	localFrag, localPwd, err := session.Agent.GetLocalUserCredentials()
// 	if err != nil {
// 		log.Error("failed to get local user credentials", logger.Err(err), receiverIdLog, userIdLog)
// 		return errs.NewAppError(op, err)
// 	}
// 	creds := fmt.Appendf(nil, "%s %s", localFrag, localPwd)
// 	sdp := utils.SetFirstByte(CREDS, creds)
// 	log.Debug("sdp (credentials) creating", receiverIdLog, userIdLog)
// 	if err := ns.signalingClient.NewSDP(ctx, sdp, []string{session.UserID}); err != nil {
// 		log.Error("failed to create new sdp (credentials)", logger.Err(err), receiverIdLog, userIdLog)
// 		return errs.NewAppError(op, err)
// 	}
// 	go func() {
// 		log.Info("candidate gathering...", receiverIdLog, userIdLog)
// 		if err := session.Agent.GatherCandidates(); err != nil {
// 			log.Error("failed to gather candidates", logger.Err(err), receiverIdLog, userIdLog)
// 			ns.disconnectSession(session)
// 		}
// 	}()
// 	return nil
// }

func (ns *networkingServ) handleConnection(session *models.Session) {
	op := "networkingServ.handleConnection"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", session.UserID)
	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
	log.Info("connection handling...", idsLog...)
	defer func() {
		//ns.disconnectSession(session)
		log.Info("connection handling stopped", idsLog...)
	}()
	go ns.proccessEventStream(session)
	go ns.receiveDatagrams(session)
	ns.receiveStreams(session)
}

func (ns *networkingServ) proccessEventStream(session *models.Session) {
	op := "networkingServ.proccessEventStream"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", session.UserID)
	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
	log.Info("event stream processing...", idsLog...)

	eventHndlr, ok := ns.onEventHandler.Load().(eventHandler)
	for {
		var event Event
		if err := session.EventDecoder.Decode(&event); err != nil {
			if cerr := utils.CheckErr(ns.closeCtx, err); cerr != nil {
				log.Error("failed to decode event", logger.Err(err), userIdLog, receiverIdLog)
			}
			ns.disconnectSession(session, false)
			return
		}
		if ok {
			eventHndlr(session.UserID, event)
			log.Info("event proccessed successfully", idsLog...)
		}
	}
}

func (ns *networkingServ) initEventStream(ctx context.Context, session *models.Session) error {
	op := "networkingServ.initEventStream"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", session.UserID)
	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
	log.Info("event stream initing...", idsLog...)
	var (
		eventStream *quic.Stream
		err         error
	)
	if session.IsInitiator {
		eventStream, err = session.Conn.OpenStreamSync(ctx)
		if err != nil {
			log.Error("failed to open event stream", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
		if _, err := eventStream.Write([]byte{0}); err != nil {
			log.Error("failed to write handshake byte to event stream", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
	} else {
		eventStream, err = session.Conn.AcceptStream(ctx)
		if err != nil {
			log.Error("failed to accept event stream", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
		h := make([]byte, 1)
		if _, err := io.ReadFull(eventStream, h); err != nil {
			log.Error("failed to read handshake byte to event stream", logger.Err(err), userIdLog, receiverIdLog)
			return errs.NewAppError(op, err)
		}
	}

	secureEventStream, err := e2ee.NewSecureStream(eventStream, session.Key)
	if err != nil {
		log.Error("failed to create secure event stream", logger.Err(err), userIdLog, receiverIdLog)
		return errs.NewAppError(op, err)
	}

	log.Info("secure event stream created", idsLog...)

	decoder := json.NewDecoder(secureEventStream)
	encoder := json.NewEncoder(secureEventStream)

	session.EventStream = eventStream
	session.EventDecoder = decoder
	session.EventEncoder = encoder

	log.Info("event stream inited successfully", idsLog...)

	return nil

}

func (ns *networkingServ) receiveStreams(session *models.Session) {
	op := "networkingServ.receiveStreams"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverid", session.UserID)
	log.Info("streams receiving...", receiverIdLog, userIdLog)
	for {
		stream, err := session.Conn.AcceptUniStream(ns.closeCtx)
		if err != nil {
			if cerr := utils.CheckErr(ns.closeCtx, err); cerr != nil {
				log.Error("failed to accept uni stream", logger.Err(err), userIdLog, receiverIdLog)
			}

			ns.disconnectSession(session, false)
			return
		}

		secureStream, err := e2ee.NewSecureStream(stream, session.Key)
		if err != nil {
			log.Error("failed to create secure stream", logger.Err(err), userIdLog, receiverIdLog)
			continue
		}

		data, err := secureStream.Receive()
		if err != nil {
			log.Error("failed to read data from secure stream", logger.Err(err), userIdLog, receiverIdLog)
			continue
		}

		if len(data) < 1 {
			log.Error("received empty data", userIdLog, receiverIdLog)
			continue
		}
		msgLenLog := logger.Attr("msgLen", len(data[1:]))
		log.Info("new msg from stream received", userIdLog, receiverIdLog, msgLenLog)
		stream.CancelRead(0)
		ns.processData(session.UserID, data)
	}
}

func (ns *networkingServ) receiveDatagrams(session *models.Session) {
	op := "networkingServ.receiveDatagrams"
	log := ns.logger.AddOp(op)
	sparseLog := log.Sparse(ns.cfg.DatagramLogTargetCount)
	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", session.UserID)
	idsLog := logger.NewLogData(userIdLog, receiverIdLog)
	var logCount uint32 = 0
	sparseLog.Info(logCount, "datagram receiving...", idsLog...)

	for {
		logCount++
		datagram, err := session.Conn.ReceiveDatagram(ns.closeCtx)
		if err != nil {
			if cerr := utils.CheckErr(ns.closeCtx, err); cerr != nil {
				sparseLog.Error(logCount, "failed to receive datagram", logger.Err(err), userIdLog, receiverIdLog)
			}
			ns.disconnectSession(session, false)
			return
		}
		data, err := e2ee.DecipherDatagram(datagram, session.Key)
		if err != nil {
			log.Error("failed to decipher datagram", logger.Err(err))
			continue
		}
		if len(data) < 1 {
			sparseLog.Error(logCount, "received empty data", userIdLog, receiverIdLog)
			continue
		}

		msgLenLog := logger.Attr("msgLen", len(data[1:]))
		sparseLog.Info(logCount, "new datagram received", userIdLog, receiverIdLog, msgLenLog)
		ns.processData(session.UserID, data)
	}
}

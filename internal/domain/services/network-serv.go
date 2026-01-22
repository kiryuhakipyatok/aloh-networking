package services

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"networking/internal/client"
	"networking/internal/config"
	"networking/internal/domain/models"
	"networking/internal/domain/repository"
	"networking/internal/protocol"
	"networking/internal/utils"
	"networking/pkg/errs"
	"networking/pkg/logger"
	"strings"
	"sync"
	"time"

	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/quic-go/quic-go"
	"golang.org/x/sync/singleflight"
)

const (
	CREDS = iota
	CANDIDATE
	CONNECTED    = "Connected"
	DISCONNECTED = "Disconnected"
	CLOSED       = "Closed"
	FAILED       = "Failed"
)

type NetworkingServ interface {
	Сonnect(rids []string) error
	SendMessage(ctx context.Context, msg string) error
	SendDatagram(ctx context.Context, data []byte) error
	Disconnect(ctx context.Context) error
	ReceiveConnects() error
}

type networkingServ struct {
	signalingClient client.SignalingClient
	sessionRepo     repository.SessionRepository
	receiveSDPs     chan protocol.ReplyMessage
	sdpsGroup       singleflight.Group
	logger          *logger.Logger
	cfg             config.Networking
	closeCtx        context.Context
}

func NewNetworkingServ(ctx context.Context, sc client.SignalingClient, cfg config.Networking, l *logger.Logger, sr repository.SessionRepository, receiveSDPs chan protocol.ReplyMessage) NetworkingServ {
	ns := &networkingServ{
		signalingClient: sc,
		sessionRepo:     sr,
		receiveSDPs:     receiveSDPs,
		sdpsGroup:       singleflight.Group{},
		cfg:             cfg,
		logger:          l,
		closeCtx:        ctx,
	}

	return ns
}

func (ns *networkingServ) Сonnect(rids []string) error {
	op := "networkingServ.Сonnect"
	log := ns.logger.AddOp(op)
	log.Info("connecting...")
	for _, rid := range rids {
		go func(rid string) {
			receiverIdLog := logger.Attr("receiverId", rid)
			session, err := ns.createSession(context.Background(), rid, true)
			if err != nil {
				log.Error("failed to craete session", logger.Err(err), receiverIdLog)
				return
			}
			go func() {
				if err := ns.establishConnection(context.Background(), session); err != nil {
					log.Error("failed to establish connection", logger.Err(err), receiverIdLog)
					ns.disconnectSession(session)
				}
			}()
		}(rid)

	}
	return nil
}

func (ns *networkingServ) Disconnect(ctx context.Context) error {
	op := "networkingServ.Disconnect"
	log := ns.logger.AddOp(op)
	log.Info("disconnecting...")
	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Error("failed to fetch sessions", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions")
		return errs.ErrNotFound(op)
	}
	for _, session := range sessions {
		ns.disconnectSession(session)
	}
	log.Info("disconnected successfully")
	return nil
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
	log.Info("creating new agent", ridLog)
	agent, err := ice.NewAgent(&ice.AgentConfig{
		Urls: []*stun.URI{
			{Scheme: stun.SchemeTypeSTUN, Host: ns.cfg.STUNHost, Port: ns.cfg.STUNPort, Proto: stun.ProtoTypeUDP},
			{Scheme: stun.SchemeTypeTURN, Host: ns.cfg.TURNHost, Port: ns.cfg.TURNPort, Username: ns.cfg.TURNUsername, Password: ns.cfg.TURNPassword, Proto: stun.ProtoTypeTCP},
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
	log.Info("session saving", ridLog)
	if err := ns.sessionRepo.Add(ctx, rid, session); err != nil {
		log.Error("failed to save session", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	localFrag, localPwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		log.Error("failed to get local user credentials", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	creds := []byte(fmt.Sprintf("%s %s", localFrag, localPwd))
	sdp := utils.SetFirstByte(CREDS, creds)
	log.Info("sdp (credentials) creating", ridLog)
	if err := ns.signalingClient.NewSDP(ctx, sdp, []string{session.UserID}); err != nil {
		log.Error("failed to create new sdp (credentials)", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}
		candidate := []byte(c.Marshal())
		sdp := utils.SetFirstByte(CANDIDATE, candidate)
		log.Info("sdp (candidate) creating", ridLog)
		if err := ns.signalingClient.NewSDP(ctx, sdp, []string{rid}); err != nil {
			log.Error("failed to create new sdp (candidate)", logger.Err(err), ridLog)
		}
	})
	if err = agent.OnConnectionStateChange(func(c ice.ConnectionState) {
		session.State = c.String()
		if c.String() == CLOSED || c.String() == DISCONNECTED || c.String() == FAILED {
			ns.disconnectSession(session)
		}
	}); err != nil {
		log.Error("failed on connection state change", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	log.Info("candidate gathering", ridLog)
	if err := agent.GatherCandidates(); err != nil {
		log.Error("failed to gather candidates", logger.Err(err), ridLog)
		return nil, errs.NewAppError(op, err)
	}
	return session, nil
}

func (ns *networkingServ) ReceiveConnects() error {
	op := "networkingServ.receiveConnect"
	log := ns.logger.AddOp(op)
	log.Info("connection receiving...")

	for sdp := range ns.receiveSDPs {
		go func(sdp protocol.ReplyMessage) {
			// ctx, cancel := context.WithTimeout(ns.closeCtx, time.Second*10)
			senderId := sdp.Sender
			senderIdLog := logger.Attr("senderId", senderId)
			log.Info("received new sdp", senderIdLog)
			v, err, _ := ns.sdpsGroup.Do(sdp.Sender, func() (any, error) {
				return ns.getSession(context.Background(), senderId)
			})
			if err != nil {
				log.Error("failed to get session", logger.Err(err), senderIdLog)
			}
			session, ok := v.(*models.Session)
			if !ok {
				log.Error("invalid session type", senderIdLog)
			}
			switch sdp.Payload[0] {
			case CREDS:
				log.Info("credentials processing", senderIdLog)
				creds := strings.Split(string(sdp.Payload[1:]), " ")
				remoteUrfrag := creds[0]
				remotePwd := creds[1]
				if err := session.Agent.SetRemoteCredentials(remoteUrfrag, remotePwd); err != nil {
					log.Error("failed to set remote credential", logger.Err(err), senderIdLog)
				}
				select {
				case session.CredsChan <- struct{}{}:
				default:
				}
				log.Info("credentials processed", senderIdLog)

			case CANDIDATE:
				log.Info("candidate processing", senderIdLog)
				c, err := ice.UnmarshalCandidate(string(sdp.Payload[1:]))
				if err != nil {
					log.Error("failed to unmarshal candidate", logger.Err(err), senderIdLog)
				}
				if err := session.Agent.AddRemoteCandidate(c); err != nil {
					log.Error("failed to add remote candidate", logger.Err(err), senderIdLog)
				}
				log.Info("candidate processed", senderIdLog)
			}
		}(sdp)
	}

	return nil

}

func (ns *networkingServ) getSession(ctx context.Context, id string) (*models.Session, error) {
	op := "networkingServ.getSession"
	log := ns.logger.AddOp(op)
	userLog := logger.Attr("userId", id)
	log.Info("session getting...", userLog)
	session, err := ns.sessionRepo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, errs.ErrNotFoundBase) {
			session, err = ns.createSession(ctx, id, false)
			if err != nil {
				log.Error("failed to create session", logger.Err(err), userLog)
			}

			go func() {
				if err := ns.establishConnection(context.Background(), session); err != nil {
					log.Error("failed to establish connection", logger.Err(err), userLog)
					ns.disconnectSession(session)
				}
			}()
		} else {
			log.Error("failed to get session", logger.Err(err), userLog)
		}
	}
	log.Info("session received successfully", userLog)
	return session, nil
}

func (ns *networkingServ) establishConnection(ctx context.Context, session *models.Session) error {
	op := "networkingServ.establishConnection"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", session.UserID)
	select {
	case <-session.CredsChan:
	case <-ctx.Done():
		log.Error("context is done", logger.Err(ctx.Err()), userIdLog)
		return errs.NewAppError(op, ctx.Err())
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
		KeepAlivePeriod: time.Second * 10,
		EnableDatagrams: true,
	}
	switch session.IsInitiator {
	case true:
		log.Info("dialing agent connection", userIdLog)
		conn, err = session.Agent.Dial(ctx, remoteUfrag, remotePwd)
		if err != nil {
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
		tlsConf := utils.GenerateTLSConfig(ns.cfg.NextProtos)
		listener, err := t.Listen(tlsConf, quicConf)
		if err != nil {
			log.Error("failed to start listen quic connections", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
		quicConn, err = listener.Accept(ctx)
		if err != nil {
			log.Error("failed to accept quic connection", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
	}
	session.Conn = quicConn
	log.Info("connection established successfully", userIdLog)
	go ns.handleConnection(ctx, session)
	return nil

}

func (ns *networkingServ) handleConnection(ctx context.Context, session *models.Session) {
	op := "networkingServ.handleConnection"
	log := ns.logger.AddOp(op)
	remoteAddrLog := logger.Attr("remoteAddr", session.Conn.RemoteAddr())
	localAddrLog := logger.Attr("localAddr", session.Conn.LocalAddr())
	connLog := logger.NewLogData(remoteAddrLog, localAddrLog)
	log.Info("connection handling...", connLog...)

	buf := make([]byte, 128)
	go func() {
		for session.State == CONNECTED {
			data, err := session.Conn.ReceiveDatagram(ctx)
			if err != nil {
				if cerr := utils.CheckErr(ctx, err); cerr == nil {
					return
				}
				log.Error("failed to receive datagram", logger.Err(err), remoteAddrLog, localAddrLog)
			}
			fmt.Println(string(data))
		}
	}()
	for session.State == CONNECTED {
		stream, err := session.Conn.AcceptUniStream(ctx)
		if err != nil {
			if cerr := utils.CheckErr(ctx, err); cerr == nil {
				return
			}
			log.Error("failed to accept uni stream", logger.Err(err), remoteAddrLog, localAddrLog)
			continue
		}
		n, err := stream.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Error("failed to read from stream", logger.Err(err), remoteAddrLog, localAddrLog)
			continue
		}
		fmt.Println(string(buf[:n]))
		stream.CancelRead(0)

	}
}

func (ns *networkingServ) SendMessage(ctx context.Context, msg string) error {
	op := "networkingServ.SendMessage"
	log := ns.logger.AddOp(op)
	log.Info("message sending")

	sessions, err := ns.sessionRepo.Fetch(context.Background())
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions")
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		go func(s *models.Session) {
			gctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()
			userIdLog := logger.Attr("userId", s.UserID)
			if s.State == CONNECTED {
				stream, err := s.Conn.OpenUniStreamSync(gctx)
				if err != nil {
					// if cerr := utils.CheckErr(gctx, err); cerr == nil {
					// 	if err := ns.sessionRepo.Delete(ctx, s.UserID); err != nil {
					// 		log.Error("failed to delete session", logger.Err(err), logger.Attr("userId", s.UserID))
					// 	}
					// } else {
					log.Error("failed to open uni stream", logger.Err(err), userIdLog)
					// }
					return
				}
				if _, err := stream.Write([]byte(msg)); err != nil {
					// fmt.Println(err.Error())
					// if cerr := utils.CheckErr(gctx, err); cerr == nil {
					// 	if err := ns.sessionRepo.Delete(ctx, s.UserID); err != nil {
					// 		log.Error("failed to delete session", logger.Err(err), logger.Attr("userId", s.UserID))
					// 	}
					// } else {
					log.Error("failed to write msg in stream", logger.Err(err), userIdLog)
					// }

					return
				}
				if err := stream.Close(); err != nil {

					if cerr := utils.CheckErr(gctx, err); cerr == nil {
						return

					}
					log.Error("failed to close uni stream", logger.Err(err), userIdLog)
					return
				}
			} else {
				log.Info("user are not connected", userIdLog)
			}
		}(s)

	}
	log.Info("message sended")
	return nil

}

func (ns *networkingServ) SendDatagram(ctx context.Context, data []byte) error {
	op := "networkingServ.SendDatagram"
	log := ns.logger.AddOp(op)
	log.Info("datagram sending")

	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions")
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		go func(s *models.Session) {
			// gctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			// defer cancel()
			userIdLog := logger.Attr("userId", s.UserID)
			if s.State == CONNECTED {
				if err := s.Conn.SendDatagram(data); err != nil {
					// if cerr := utils.CheckErr(gctx, err); cerr == nil {
					// 	if err := ns.sessionRepo.Delete(ctx, s.UserID); err != nil {
					// 		log.Error("failed to delete session", logger.Err(err), logger.Attr("userId", s.UserID))
					// 	}
					// 	log.Info("disconnected", userIdLog)
					// } else {
					log.Info("failed to send datagram", logger.Err(err), userIdLog)
					// }

					return
				}
			} else {
				log.Info("user are not connected", userIdLog)
			}

		}(s)

	}
	log.Info("datagram sended")
	return nil

}

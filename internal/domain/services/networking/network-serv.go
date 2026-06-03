package networking

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/kiryuhakipyatok/aloh-networking/config"
	"github.com/kiryuhakipyatok/aloh-networking/internal/client"

	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/e2ee"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/models"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/repository"
	"github.com/kiryuhakipyatok/aloh-networking/internal/utils"
	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"
	"github.com/pion/ice/v2"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

const (
	CREDS = iota
	CANDIDATE

	CONNECTED    = ice.ConnectionStateConnected
	DISCONNECTED = ice.ConnectionStateDisconnected
	CLOSED       = ice.ConnectionStateClosed
	FAILED       = ice.ConnectionStateFailed
)

const (
	CHAT = iota
	VOICE
	VIDEO
)

const (
	INITIATOR     = true
	NOT_INITIATOR = false
)

type NetworkingServ interface {
	Connect(ctx context.Context, rid string) error
	ConnectById(ctx context.Context, rid string) error
	Disconnect() error
	DisconnectFromId(ctx context.Context, sessionId string) error
	SendInEventStream(ctx context.Context, e Event) error
	SendInStream(ctx context.Context, data []byte) error
	SendDatagram(ctx context.Context, data []byte) error
	SaveChatHandler(h dataHandler)
	SaveVoiceHandler(h dataHandler)
	SaveVideoHandler(h dataHandler)
	SavePeerConnectedHandler(h connectionHandler)
	SavePeerDisconnectedHandler(h connectionHandler)
	SaveEventHandler(h eventHandler)
	FetchOnline(ctx context.Context) ([]string, error)
	FetchSessionsById(ctx context.Context, id string) ([]string, error)
	FetchOnlineFriends(ctx context.Context, friends []string) (map[string][]string, error)
}

type networkingServ struct {
	id                   string
	signalingClient      client.SignalingClient
	sessionRepo          repository.SessionRepository
	receiveSDPs          chan client.ReplyMessage
	sdpsGroup            singleflight.Group
	logger               *logger.Logger
	cfg                  config.Networking
	closeCtx             context.Context
	tlsConf              *tls.Config
	sendDatagramLogCount atomic.Uint32
	fetchLogCount        atomic.Uint32
	handlers
}

type NewNetworkingSetup struct {
	Id          string
	SC          client.SignalingClient
	Cfg         config.Networking
	L           *logger.Logger
	SR          repository.SessionRepository
	ReceiveSDPs chan client.ReplyMessage
}

func NewNetworkingServ(ctx context.Context, setup NewNetworkingSetup) NetworkingServ {
	closeCtx, cancel := context.WithCancel(ctx)
	tlsConf := utils.GenerateTLSConfig(setup.Cfg.NextProtos)
	ns := &networkingServ{
		id:              setup.Id,
		signalingClient: setup.SC,
		sessionRepo:     setup.SR,
		receiveSDPs:     setup.ReceiveSDPs,
		sdpsGroup:       singleflight.Group{},
		cfg:             setup.Cfg,
		logger:          setup.L,
		closeCtx:        closeCtx,
		tlsConf:         tlsConf,
	}

	ns.sendDatagramLogCount.Store(0)
	ns.sendDatagramLogCount.Store(1)

	go func() {
		if err := ns.receiveConnects(); err != nil {
			ns.logger.Info("failed to receive connects", logger.Err(err))
			cancel()
		}
	}()

	return ns
}

func (ns *networkingServ) Connect(ctx context.Context, rid string) error {
	op := "networkingServ.Connect"
	log := ns.logger.AddOp(op)
	log.Info("connecting...")
	userIdLog := logger.Attr("userId", ns.id)
	mergeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-mergeCtx.Done():
		case <-ns.closeCtx.Done():
			cancel()
		}
	}()
	g, gCtx := errgroup.WithContext(mergeCtx)

	if rid == ns.id {
		errMsg := "cannot connect to himself"
		log.Error(errMsg, userIdLog)
		return errors.New(errMsg)
	}

	curSessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Error("failed to fetch current sessions", userIdLog)
		return errs.NewAppError(op, err)
	}

	if len(curSessions) > 0 {
		log.Info("reconnecting...")
		if err = ns.Disconnect(); err != nil {
			log.Error("failed to disconnect", logger.Err(err), userIdLog)
			return errs.NewAppError(op, err)
		}
	}

	g.Go(func() error {
		receiverIdLog := logger.Attr("receiverId", rid)

		session, err := ns.createAndEstablish(gCtx, rid, INITIATOR)
		if err != nil {
			log.Error("failed to create and establish connection", logger.Err(err), receiverIdLog)
			return errs.NewAppError(op, err)
		}
		<-session.ReadyChan
		return nil
	})
	receiversSessions, err := ns.signalingClient.GetSessionsById(ctx, rid)
	if err != nil {
		return errs.NewAppError(op, err)
	}

	var resultReceiversSessions []string
	err = json.Unmarshal(receiversSessions, &resultReceiversSessions)
	if err != nil {
		return errs.NewAppError(op, err)
	}

	if len(resultReceiversSessions) > 0 {
		for _, ss := range resultReceiversSessions {
			if ss != ns.id {
				g.Go(func() error {
					receiverIdLog := logger.Attr("receiverId", ss)
					session, err := ns.createAndEstablish(gCtx, ss, INITIATOR)
					if err != nil {
						log.Error("failed to create and establish connection", logger.Err(err), receiverIdLog)
						return errs.NewAppError(op, err)
					}
					<-session.ReadyChan
					return nil
				})
			}
		}
	}

	if err := g.Wait(); err != nil {
		return errs.NewAppError(op, err)
	}
	return nil
}

func (ns *networkingServ) ConnectById(ctx context.Context, rid string) error {
	op := "networkingServ.ConnectById"
	log := ns.logger.AddOp(op)

	userIdLog := logger.Attr("userId", ns.id)
	receiverIdLog := logger.Attr("receiverId", rid)
	log.Info("connecting by id...", userIdLog, receiverIdLog)
	mergeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-mergeCtx.Done():
		case <-ns.closeCtx.Done():
			cancel()
		}
	}()

	if rid == ns.id {
		return errs.ErrConnToHimself(op)
	}

	session, err := ns.createAndEstablish(mergeCtx, rid, INITIATOR)
	if err != nil {
		log.Error("failed to create and establish connection", logger.Err(err), userIdLog, receiverIdLog)
		return errs.NewAppError(op, err)
	}
	<-session.ReadyChan
	log.Info("connected by id successfully", userIdLog, receiverIdLog)
	return nil
}

func (ns *networkingServ) Disconnect() error {
	op := "networkingServ.Disconnect"
	log := ns.logger.AddOp(op)
	log.Info("disconnecting...")
	var wg sync.WaitGroup
	sessions, err := ns.sessionRepo.Fetch(context.Background())
	if err != nil {
		log.Error("failed to fetch sessions", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions")
		return nil
	}
	for _, session := range sessions {
		wg.Go(func() {
			ns.disconnectSession(session, true)
		})
	}
	wg.Wait()
	log.Info("disconnected successfully")
	return nil
}

func (ns *networkingServ) DisconnectFromId(ctx context.Context, sessionId string) error {
	op := "networkingServ.DisconnectFrom"
	log := ns.logger.AddOp(op)
	sessionIdLog := logger.Attr("sessionId", sessionId)
	log.Info("disconnecting from session", sessionIdLog)
	session, err := ns.sessionRepo.Get(ctx, sessionId)
	if err != nil {
		log.Error("failed to get session", logger.Err(err), sessionIdLog)
		return errs.NewAppError(op, err)
	}

	ns.disconnectSession(session, true)

	log.Info("disconnected from session successfully")
	return nil
}

func (ns *networkingServ) SendInEventStream(ctx context.Context, e Event) error {
	op := "networkingServ.SendInEventStream"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	log.Info("event sending...", userIdLog)
	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err), userIdLog)
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions", logger.Err(err), userIdLog)
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		if s.Conn != nil && s.EventStream != nil {
			go func(s *models.Session) {
				recIdLog := logger.Attr("recieverId", s.UserID)
				{
					if err := s.EventEncoder.Encode(e); err != nil {
						log.Error("failed to encode event", logger.Err(err), recIdLog, userIdLog)
						return
					}
				}
			}(s)
		}
	}
	log.Info("event sent", userIdLog)
	return nil
}

func (ns *networkingServ) SendInStream(ctx context.Context, data []byte) error {
	op := "networkingServ.SendMessage"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.id)
	payload := data[1:]
	msgLenLog := logger.Attr("msgLen", len(payload))
	sendMsgLog := logger.NewLogData(userIdLog, msgLenLog)
	log.Info("message sending", sendMsgLog...)

	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err), msgLenLog, userIdLog)
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions", logger.Err(err), msgLenLog, userIdLog)
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		if s.Conn != nil {
			select {
			case <-s.ReadyChan:
				go func(s *models.Session) {
					gctx, cancel := context.WithTimeout(context.Background(), ns.cfg.SendInStreamTimeout)
					defer cancel()
					recIdLog := logger.Attr("recieverId", s.UserID)
					{
						stream, err := s.Conn.OpenUniStreamSync(gctx)
						if err != nil {
							log.Error("failed to open uni stream", logger.Err(err), recIdLog, userIdLog, msgLenLog)

							return
						}

						secureStream, err := e2ee.NewSecureStream(stream, s.Key)
						if err != nil {
							log.Error("failed to create new secure stream", logger.Err(err), recIdLog, userIdLog, msgLenLog)
							return
						}

						if err := secureStream.Send(data); err != nil {
							log.Error("failed to send data in secure stream", logger.Err(err), recIdLog, userIdLog, msgLenLog)
							return
						}

						if err := secureStream.Close(); err != nil {
							if cerr := utils.CheckErr(gctx, err); cerr == nil {
								return

							}
							log.Error("failed to close secure stream", logger.Err(err), recIdLog, userIdLog, msgLenLog)
							return
						}

					}
				}(s)
			default:
			}

		}
	}
	log.Info("message sent", sendMsgLog...)
	return nil

}

func (ns *networkingServ) SendDatagram(ctx context.Context, data []byte) error {
	ns.sendDatagramLogCount.Add(1)
	op := "networkingServ.SendDatagram"
	log := ns.logger.AddOp(op)
	sparseLog := log.Sparse(ns.cfg.DatagramLogTargetCount)
	userIdLog := logger.Attr("userId", ns.id)
	dgLenLog := logger.Attr("msgLen", len(data[1:]))
	sendDatagramLog := logger.NewLogData(userIdLog, dgLenLog)
	sparseLog.Info(ns.sendDatagramLogCount.Load(), "datagram sending", sendDatagramLog...)

	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		sparseLog.Error(ns.sendDatagramLogCount.Load(), "failed to fetch sessions", logger.Err(err), dgLenLog, userIdLog)
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		sparseLog.Info(ns.sendDatagramLogCount.Load(), "zero sessions", sendDatagramLog...)
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		if s.Conn != nil {
			select {
			case <-s.ReadyChan:
				go func(s *models.Session) {

					recIdLog := logger.Attr("receiverId", s.UserID)

					cipherDatagram, err := e2ee.CipherDatagram(data, s.Key)
					if err != nil {
						log.Error("failed to cipher datagram", logger.Err(err), userIdLog, recIdLog, dgLenLog)
						return
					}
					if err := s.Conn.SendDatagram(cipherDatagram); err != nil {
						log.Error("failed to send datagram", logger.Err(err), userIdLog, recIdLog, dgLenLog)
						return
					}
					sparseLog.Info(ns.sendDatagramLogCount.Load(), "datagram sent", sendDatagramLog...)

				}(s)
			default:
			}
		}
	}

	return nil
}

func (ns *networkingServ) FetchOnline(ctx context.Context) ([]string, error) {
	ns.fetchLogCount.Add(1)
	op := "networkingServ.FetchOnline"
	log := ns.logger.AddOp(op)
	sparceLog := log.Sparse(ns.cfg.FetchLogTargetCount)
	sparceLog.Info(ns.fetchLogCount.Load(), "online fetching...")
	onlineIdsByte, err := ns.signalingClient.GetOnline(ctx)
	if err != nil {
		log.Error("failed to fetch online", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	var onlineIds []string
	if err := json.Unmarshal(onlineIdsByte, &onlineIds); err != nil {
		log.Error("failed to unmarshal online", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	sparceLog.Info(ns.fetchLogCount.Load(), "online fetched successfully")
	return onlineIds, nil

}

func (ns *networkingServ) FetchOnlineFriends(ctx context.Context, friends []string) (map[string][]string, error) {
	ns.fetchLogCount.Add(1)
	op := "networkingServ.FetchOnlineFriends"
	log := ns.logger.AddOp(op)
	sparceLog := log.Sparse(ns.cfg.FetchLogTargetCount)
	sparceLog.Info(ns.fetchLogCount.Load(), "online friends fetching...")
	onlineFriendsByte, err := ns.signalingClient.GetFriendsOnline(ctx, friends)
	if err != nil {
		log.Error("failed to fetch online friends", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	var onlineFriends map[string][]string
	if err := json.Unmarshal(onlineFriendsByte, &onlineFriends); err != nil {
		log.Error("failed to unmarshal online friends", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	sparceLog.Info(ns.fetchLogCount.Load(), "online friends fetched successfully")
	return onlineFriends, nil

}

func (ns *networkingServ) FetchSessionsById(ctx context.Context, id string) ([]string, error) {
	ns.fetchLogCount.Add(1)
	op := "networkingServ.FetchSessions"
	log := ns.logger.AddOp(op)
	sparceLog := log.Sparse(ns.cfg.FetchLogTargetCount)
	sparceLog.Info(ns.fetchLogCount.Load(), "sessions by id fetching...")
	sessionsByte, err := ns.signalingClient.GetSessionsById(ctx, id)
	if err != nil {
		log.Error("failed to fetch sessions by id", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	var sessions []string
	if err := json.Unmarshal(sessionsByte, &sessions); err != nil {
		log.Error("failed to unmarshal sessions by id", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}
	sparceLog.Info(ns.fetchLogCount.Load(), "sessions by id fetched successfully")
	return sessions, nil
}

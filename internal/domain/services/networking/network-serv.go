package networking

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"networking/config"
	"networking/internal/client"
	"networking/internal/domain/models"
	"networking/internal/domain/repository"
	"networking/internal/utils"
	"networking/pkg/errs"
	"networking/pkg/logger"
	"sync"

	"golang.org/x/sync/errgroup"
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

const (
	CHAT = iota
	VOICE
	VIDEO
)

type NetworkingServ interface {
	Connect(ctx context.Context, rids []string) error
	Disconnect() error
	SendInStream(ctx context.Context, data []byte) error
	SendDatagram(ctx context.Context, data []byte) error
	SaveChatHandler(h handler)
	SaveVoiceHandler(h handler)
	SaveVideoHandler(h handler)
	FetchOnline(ctx context.Context) ([]string, error)
	FetchSessionsById(ctx context.Context, id string) ([]string, error)
}

type networkingServ struct {
	userId          string
	signalingClient client.SignalingClient
	sessionRepo     repository.SessionRepository
	receiveSDPs     chan client.ReplyMessage
	sdpsGroup       singleflight.Group
	logger          *logger.Logger
	cfg             config.Networking
	closeCtx        context.Context
	tlsConf         *tls.Config
	handlers
}

func NewNetworkingServ(ctx context.Context, id string, sc client.SignalingClient, cfg config.Networking, l *logger.Logger, sr repository.SessionRepository, receiveSDPs chan client.ReplyMessage) NetworkingServ {
	closeCtx, cancel := context.WithCancel(ctx)
	tlsConf := utils.GenerateTLSConfig(cfg.NextProtos)
	ns := &networkingServ{
		userId:          id,
		signalingClient: sc,
		sessionRepo:     sr,
		receiveSDPs:     receiveSDPs,
		sdpsGroup:       singleflight.Group{},
		cfg:             cfg,
		logger:          l,
		closeCtx:        closeCtx,
		tlsConf:         tlsConf,
	}

	go func() {
		if err := ns.receiveConnects(); err != nil {
			ns.logger.Info("failed to receive connects", logger.Err(err))
			cancel()
		}
	}()

	return ns
}

func (ns *networkingServ) Connect(ctx context.Context, rids []string) error {
	op := "networkingServ.Connect"
	log := ns.logger.AddOp(op)
	log.Info("connecting...")
	userIdLog := logger.Attr("userId", ns.userId)
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
	for _, rid := range rids {
		if rid == ns.userId {
			errMsg := "cannot connect to himself"
			log.Error(errMsg, userIdLog)
			return errors.New(errMsg)
		}
		g.Go(func() error {
			receiverIdLog := logger.Attr("receiverId", rid)

			session, err := ns.createSession(gCtx, rid, true)
			if err != nil {
				log.Error("failed to create session", logger.Err(err), userIdLog, receiverIdLog)
				return err
			}
			estCtx, cancel := context.WithTimeout(gCtx, ns.cfg.EstablishConnTimeout)
			defer cancel()
			if err := ns.establishConnection(estCtx, session); err != nil {
				log.Error("failed to establish connection", logger.Err(err), userIdLog, receiverIdLog)
				ns.disconnectSession(session)
				return err
			}

			return nil
		})

	}
	if err := g.Wait(); err != nil {
		return errs.NewAppError(op, err)
	}
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
			ns.disconnectSession(session)
		})
	}
	wg.Wait()
	log.Info("disconnected successfully")
	return nil
}

func (ns *networkingServ) SendInStream(ctx context.Context, data []byte) error {
	op := "networkingServ.SendMessage"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.userId)
	msgLog := logger.Attr("msg", string(data))
	msgLenLog := logger.Attr("msgLen", len(data))
	sendMsgLog := logger.NewLogData(userIdLog, msgLenLog, msgLog)
	log.Info("message sending", sendMsgLog...)

	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err), msgLenLog, msgLog, userIdLog)
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions", logger.Err(err), msgLenLog, msgLog, userIdLog)
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		if s.Conn != nil {
			go func(s *models.Session) {
				gctx, cancel := context.WithTimeout(context.Background(), ns.cfg.SendInStreamTimeout)
				defer cancel()
				recIdLog := logger.Attr("recieverId", s.UserID)
				{
					stream, err := s.Conn.OpenUniStreamSync(gctx)
					if err != nil {
						log.Error("failed to open uni stream", logger.Err(err), recIdLog, userIdLog, msgLenLog, msgLog)

						return
					}

					if _, err := stream.Write(data); err != nil {
						log.Error("failed to write msg in stream", logger.Err(err), recIdLog, userIdLog, msgLenLog, msgLog)
						return
					}
					if err := stream.Close(); err != nil {

						if cerr := utils.CheckErr(gctx, err); cerr == nil {
							return

						}
						log.Error("failed to close uni stream", logger.Err(err), recIdLog, userIdLog, msgLenLog, msgLog)
						return
					}
				}
			}(s)
		}
	}
	log.Info("message sent", userIdLog)
	return nil

}

func (ns *networkingServ) SendDatagram(ctx context.Context, data []byte) error {
	op := "networkingServ.SendDatagram"
	log := ns.logger.AddOp(op)
	userIdLog := logger.Attr("userId", ns.userId)
	dgLenLog := logger.Attr("msgLen", len(data))
	sendDatagramLog := logger.NewLogData(userIdLog, dgLenLog)
	log.Info("datagram sending", sendDatagramLog...)

	sessions, err := ns.sessionRepo.Fetch(ctx)
	if err != nil {
		log.Info("failed to fetch sessions", logger.Err(err), dgLenLog, userIdLog)
		return errs.NewAppError(op, err)
	}
	if len(sessions) == 0 {
		log.Info("zero sessions", sendDatagramLog...)
		return errs.ErrNotFound(op)
	}
	for _, s := range sessions {
		if s.Conn != nil {
			go func(s *models.Session) {

				recIdLog := logger.Attr("receiverId", s.UserID)

				if err := s.Conn.SendDatagram(data); err != nil {
					log.Info("failed to send datagram", logger.Err(err), userIdLog, recIdLog, dgLenLog)
					return
				}

			}(s)
		}
	}
	log.Info("datagram sent", sendDatagramLog...)
	return nil

}

func (ns *networkingServ) FetchOnline(ctx context.Context) ([]string, error) {
	op := "networkingServ.FetchOnline"
	log := ns.logger.AddOp(op)
	log.Info("online fetching...")
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
	log.Info("online fetched successfully")
	return onlineIds, nil

}

func (ns *networkingServ) FetchSessionsById(ctx context.Context, id string) ([]string, error) {
	op := "networkingServ.FetchSessions"
	log := ns.logger.AddOp(op)
	log.Info("sessions by id fetching...")
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
	log.Info("sessions by id fetched successfully")
	return sessions, nil
}

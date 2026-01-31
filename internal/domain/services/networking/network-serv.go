package networking

import (
	"context"
	"errors"
	"networking/internal/client"
	"networking/internal/config"
	"networking/internal/domain/models"
	"networking/internal/domain/repository"
	"networking/internal/protocol"
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
	Сonnect(ctx context.Context, rids []string) error
	Disconnect() error
	SendInStream(ctx context.Context, data []byte) error
	SendDatagram(ctx context.Context, data []byte) error
	SaveChatHandler(h handler)
	SaveVoiceHandler(h handler)
	SaveVideoHandler(h handler)
}

type networkingServ struct {
	userId          string
	signalingClient client.SignalingClient
	sessionRepo     repository.SessionRepository
	receiveSDPs     chan protocol.ReplyMessage
	sdpsGroup       singleflight.Group
	logger          *logger.Logger
	cfg             config.Networking
	closeCtx        context.Context
	handlers
}

func NewNetworkingServ(ctx context.Context, id string, sc client.SignalingClient, cfg config.Networking, l *logger.Logger, sr repository.SessionRepository, receiveSDPs chan protocol.ReplyMessage) NetworkingServ {
	closeCtx, cancel := context.WithCancel(ctx)
	ns := &networkingServ{
		userId:          id,
		signalingClient: sc,
		sessionRepo:     sr,
		receiveSDPs:     receiveSDPs,
		sdpsGroup:       singleflight.Group{},
		cfg:             cfg,
		logger:          l,
		closeCtx:        closeCtx,
	}

	go func() {
		if err := ns.receiveConnects(); err != nil {
			ns.logger.Info("failed to receive connects", logger.Err(err))
			cancel()
		}
	}()

	return ns
}

func (ns *networkingServ) Сonnect(ctx context.Context, rids []string) error {
	op := "networkingServ.Сonnect"
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
			return errs.NewAppError(op, errors.New(errMsg))
		}
		g.Go(func() error {
			receiverIdLog := logger.Attr("receiverId", rid)
			connLog := logger.NewLogData(receiverIdLog, userIdLog)

			session, err := ns.createSession(gCtx, rid, true)
			if err != nil {
				log.Error("failed to create session", logger.Err(err), connLog)
				return err
			}

			if err := ns.establishConnection(gCtx, session); err != nil {
				log.Error("failed to establish connection", logger.Err(err), connLog)
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
		wg.Add(1)
		wg.Go(func() {
			defer wg.Done()
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
	log.Info("message sending")

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
			gctx, cancel := context.WithTimeout(context.Background(), ns.cfg.SendInStreamTimeout)
			defer cancel()
			userIdLog := logger.Attr("userId", s.UserID)
			if s.State == CONNECTED {
				stream, err := s.Conn.OpenUniStreamSync(gctx)
				if err != nil {
					log.Error("failed to open uni stream", logger.Err(err), userIdLog)

					return
				}
				if _, err := stream.Write(data); err != nil {
					log.Error("failed to write msg in stream", logger.Err(err), userIdLog)
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

			userIdLog := logger.Attr("userId", s.UserID)
			if s.State == CONNECTED {
				if err := s.Conn.SendDatagram(data); err != nil {
					log.Info("failed to send datagram", logger.Err(err), userIdLog)
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

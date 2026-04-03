package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/kiryuhakipyatok/aloh-networking/config"
	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"

	"github.com/quic-go/quic-go"
)

type SignalingClient interface {
	Close(code uint, desc string) error
	NewSDP(ctx context.Context, sdp []byte, ids []string) error
	GetOnline(ctx context.Context) ([]byte, error)
	GetSessionsById(ctx context.Context, id string) ([]byte, error)
	AddInSession(ctx context.Context, id string) error
	DeleteFromSession(ctx context.Context, id string) error
	GetCreds(ctx context.Context) (string, string, error)
}

type signalingClient struct {
	conn             *quic.Conn
	ctrlStream       *quic.Stream
	decoder          *json.Decoder
	encoder          *json.Encoder
	sendMsgs         chan Message
	receiveSDPs      chan ReplyMessage
	logger           *logger.Logger
	closeCtx         context.Context
	pendingResponses sync.Map
	username         string
	password         string
}

func NewSignalingClient(ctx context.Context, l *logger.Logger, id string, sendMsgs chan Message, receiveSDPs chan ReplyMessage, cfg config.Signaling) (SignalingClient, error) {
	op := "signalingClient.NewSignalingClient"
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         cfg.NextProtos,
	}
	quicConf := &quic.Config{

		HandshakeIdleTimeout:  cfg.HandshakeTimeout,
		MaxIdleTimeout:        cfg.IdleTimeout,
		MaxIncomingStreams:    cfg.MaxIncomingStreams,
		MaxIncomingUniStreams: cfg.MaxIncomingUniStreams,
		KeepAlivePeriod:       cfg.KeepAlivePeriodTimeout,
		EnableDatagrams:       true,
	}
	addr := fmt.Sprintf("%s:%s", cfg.Address, cfg.Port)
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, quicConf)
	if err != nil {
		return nil, errs.NewAppError(op, err)
	}
	sc := &signalingClient{
		conn:        conn,
		sendMsgs:    sendMsgs,
		receiveSDPs: receiveSDPs,
		logger:      l,
		closeCtx:    ctx,
	}
	regCtx, cancel := context.WithTimeout(ctx, cfg.RegTimeout)
	defer cancel()
	if err := sc.registerConnect(regCtx, id); err != nil {
		return nil, errs.NewAppError(op, err)
	}
	go sc.sendMsg()
	go sc.receiveSDP()
	go sc.receiveResponses()
	return sc, nil
}

func (sc *signalingClient) Close(code uint, desc string) error {
	var (
		op  = "signalingClient.Close"
		log = sc.logger.AddOp(op)
	)
	log.Info("siganling client closing...")
	if err := sc.ctrlStream.Close(); err != nil {
		log.Info("failed to close control stream", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	if err := sc.conn.CloseWithError(quic.ApplicationErrorCode(code), desc); err != nil {
		log.Info("failed to close quic connection", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	log.Info("signaling client closed successfully")
	return nil
}

func (sc *signalingClient) NewSDP(ctx context.Context, sdp []byte, ids []string) error {
	var (
		op  = "signalingClient.NewSDP"
		log = sc.logger.AddOp(op)
	)
	log.Info("creating new sdp...")

	if err := sc.sendPayload(ctx, ids, sdp); err != nil {
		log.Error("failed to send payload", logger.Err(err))
	}
	log.Info("sdp created successfully")
	return nil

}

func (sc *signalingClient) GetOnline(ctx context.Context) ([]byte, error) {
	var (
		op  = "signalingClient.GetOnline"
		log = sc.logger.AddOp(op)
	)
	log.Info("fetching online from signaling...")

	online, err := sc.getPayload(ctx, GET_ONLINE_TYPE, nil)
	if err != nil {
		log.Error("failed to get payload from signaling")
		return nil, errs.NewAppError(op, err)
	}

	log.Info("online fetched from signaling")
	return online, nil
}

func (sc *signalingClient) GetSessionsById(ctx context.Context, id string) ([]byte, error) {

	var (
		op  = "signalingClient.GetSessionsById"
		log = sc.logger.AddOp(op)
	)
	log.Info("fetching sessions by id from signaling...")
	userId := UserId{
		ID: id,
	}
	sendData, err := json.Marshal(userId)
	if err != nil {
		return nil, errs.NewAppError(op, err)
	}
	sessions, err := sc.getPayload(ctx, GET_SESSIONS_BY_ID, sendData)
	if err != nil {
		log.Error("failed to get sessions by id from signaling", logger.Err(err))
		return nil, errs.NewAppError(op, err)
	}

	log.Info("sessions by id fetched from signaling")
	return sessions, nil
}

func (sc *signalingClient) AddInSession(ctx context.Context, id string) error {

	var (
		op  = "signalingClient.AddInSession"
		log = sc.logger.AddOp(op)
	)
	log.Info("adding session to signaling...")
	userId := UserId{
		ID: id,
	}
	data, err := json.Marshal(userId)
	if err != nil {
		return errs.NewAppError(op, err)
	}

	if err := sc.sendCommand(ctx, ADD_IN_SESSION, data); err != nil {
		log.Error("failed to send command", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	return nil
}

func (sc *signalingClient) DeleteFromSession(ctx context.Context, id string) error {
	var (
		op  = "signalingClient.DeleteFromSession"
		log = sc.logger.AddOp(op)
	)
	log.Info("deleting user from session to signaling...")
	userId := UserId{
		ID: id,
	}
	data, err := json.Marshal(userId)
	if err != nil {
		return errs.NewAppError(op, err)
	}

	if err := sc.sendCommand(ctx, DELETE_FROM_SESSION, data); err != nil {
		log.Error("failed to send command", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	return nil
}

func (sc *signalingClient) GetCreds(ctx context.Context) (string, string, error) {
	var (
		op  = "signalingClient.GetCreds"
		log = sc.logger.AddOp(op)
	)
	select {
	case <-ctx.Done():
		return "", "", errs.ErrRequestTimeout(op)
	default:
	}
	log.Info("fetchig creds...")

	return sc.username, sc.password, nil
}

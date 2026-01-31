package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"networking/internal/config"
	"networking/internal/protocol"
	"networking/internal/utils"
	"networking/pkg/errs"
	"networking/pkg/logger"
	"sync"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type SignalingClient interface {
	Close(code uint, desc string)
	NewSDP(ctx context.Context, sdp []byte, ids []string) error
}

type signalingClient struct {
	conn             *quic.Conn
	ctrlStream       *quic.Stream
	decoder          *json.Decoder
	encoder          *json.Encoder
	sendSDPs         chan protocol.Message
	receiveSDPs      chan protocol.ReplyMessage
	logger           *logger.Logger
	closeCtx         context.Context
	pendingResponses sync.Map
}

func NewSignalingClient(ctx context.Context, l *logger.Logger, id string, sendSDPs chan protocol.Message, receiveSDPs chan protocol.ReplyMessage, cfg config.Signaling) SignalingClient {
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
		panic(fmt.Errorf("failed to dial quic signaling: %w", err))
	}
	sc := &signalingClient{
		conn:        conn,
		sendSDPs:    sendSDPs,
		receiveSDPs: receiveSDPs,
		logger:      l,
		closeCtx:    ctx,
	}
	regCtx, cancel := context.WithTimeout(ctx, cfg.RegTimeout)
	defer cancel()
	if err := sc.registerConnect(regCtx, id); err != nil {
		panic(fmt.Errorf("failed to register connect: %w", err))
	}
	go sc.sendSDP()
	go sc.receiveSDP()
	go sc.receiveResponses()
	return sc
}

func (sc *signalingClient) Close(code uint, desc string) {
	var (
		op  = "signalingClient.Close"
		log = sc.logger.AddOp(op)
	)
	log.Info("siganling client closing...")
	if err := sc.ctrlStream.Close(); err != nil {
		log.Info("failed to close control stream", logger.Err(err))
		panic(err)
	}
	if err := sc.conn.CloseWithError(quic.ApplicationErrorCode(code), desc); err != nil {
		log.Info("failed to close quic connection", logger.Err(err))
		panic(err)
	}
	log.Info("signaling client closed successfully")
}

func (sc *signalingClient) registerConnect(ctx context.Context, id string) error {
	var (
		op    = "signalingClient.RegisterConnect"
		log   = sc.logger.AddOp(op)
		idLog = logger.Attr("user-id", id)
	)
	log.Info("connect registering...", idLog)

	ctrlStream, err := sc.conn.OpenStreamSync(ctx)
	if err != nil {
		log.Error("failed to open quic uni stream", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	var (
		encoder = json.NewEncoder(ctrlStream)
		decoder = json.NewDecoder(ctrlStream)
	)
	sc.decoder = decoder
	sc.encoder = encoder
	sc.ctrlStream = ctrlStream
	regMsg := protocol.RegisterConnectMessage{
		ID: id,
	}
	dataReg, err := json.Marshal(regMsg)
	if err != nil {
		log.Error("failed to marshal register message", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	msg := protocol.Message{
		Id:   uuid.NewString(),
		Type: utils.Uint8ToPtr(0),
		Data: json.RawMessage(dataReg),
	}
	if err := encoder.Encode(msg); err != nil {
		log.Error("failed to encode register message to signaling", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}

	var responseMsg protocol.ResponseMessage
	if err := sc.decoder.Decode(&responseMsg); err != nil {
		log.Error("failed to decode response message from signaling", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	switch responseMsg.Code {
	case 0:
		log.Info("connect registered successfully", idLog)
		return nil
	default:
		err = errors.New(responseMsg.Msg)
		log.Error("failed to register connect", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}

}

func (sc *signalingClient) receiveResponses() {
	var (
		op  = "signalingClient.receiveResponses"
		log = sc.logger.AddOp(op)
	)
	for {
		select {
		case <-sc.closeCtx.Done():
			return
		default:
		}
		var responseMsg protocol.ResponseMessage
		if err := sc.decoder.Decode(&responseMsg); err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to receive response message from signaling", logger.Err(err))
			continue
		}

		respLog := logger.NewLogData(logger.Attr("msgId", responseMsg.MessageId), logger.Attr("respCode", responseMsg.Code), logger.Attr("respMsg", responseMsg.Msg))
		log.Info("new response message from signaling", respLog...)
		resChanInt, ok := sc.pendingResponses.Load(responseMsg.MessageId)
		if ok {
			reqChan, _ := resChanInt.(chan error)
			switch responseMsg.Code {
			case 0:
				reqChan <- nil
			default:
				reqChan <- errors.New(responseMsg.Msg)
			}
		}
	}
}

func (sc *signalingClient) sendSDP() {
	var (
		op  = "signalingClient.sendSDP"
		log = sc.logger.AddOp(op)
	)

	for sdp := range sc.sendSDPs {
		select {
		case <-sc.closeCtx.Done():
			return
		default:
		}
		msgIdLog := logger.Attr("msgId", sdp.Id)
		if err := sc.encoder.Encode(sdp); err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to send sdp to signaling", logger.Err(err), msgIdLog)
		} else {
			log.Info("sdp sended successfully", msgIdLog)
		}
	}

}

func (sc *signalingClient) receiveSDP() {
	var (
		op  = "signalingClient.sendSDP"
		log = sc.logger.AddOp(op)
	)

	for {
		stream, err := sc.conn.AcceptUniStream(sc.closeCtx)
		if err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to accept uni stream from signaling", logger.Err(err))
		}
		streamIdLog := logger.Attr("streamId", stream.StreamID())
		data, err := io.ReadAll(stream)
		if err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to read data from uni stream", logger.Err(err), streamIdLog)
		}
		sdpMsg, err := protocol.ToReplyMessage(data)
		if err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed cast sdp message to reply message", logger.Err(err), streamIdLog)
		}
		sc.receiveSDPs <- *sdpMsg
		log.Info("sdp received successfully", logger.Attr("senderId", sdpMsg.Sender))
	}

}

func (sc *signalingClient) NewSDP(ctx context.Context, sdp []byte, ids []string) error {
	var (
		op  = "signalingClient.NewSDP"
		log = sc.logger.AddOp(op)
	)
	log.Info("creating new sdp...")

	sendMsg := protocol.SendPayloadMessage{
		RecevierIDs: ids,
		Payload:     sdp,
	}
	sendData, err := json.Marshal(sendMsg)
	if err != nil {
		log.Error("failed to marshal sdp message", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	msgId := uuid.NewString()
	respChan := make(chan error, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := protocol.Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(1),
		Data: sendData,
	}
	select {
	case sc.sendSDPs <- msg:
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	}
	select {
	case err := <-respChan:
		if err != nil {
			return errs.NewAppError(op, err)
		}
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	case <-sc.closeCtx.Done():
		return nil
	}

	log.Info("sdp created successfully")
	return nil

}

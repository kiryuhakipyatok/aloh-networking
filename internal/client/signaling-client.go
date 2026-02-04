package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"networking/internal/config"
	"networking/internal/utils"
	"networking/pkg/errs"
	"networking/pkg/logger"
	"sync"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type SignalingClient interface {
	Close(code uint, desc string) error
	NewSDP(ctx context.Context, sdp []byte, ids []string) error
	GetOnline(ctx context.Context) ([]byte, error)
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
}

func NewSignalingClient(ctx context.Context, l *logger.Logger, id string, sendMsgs chan Message, receiveSDPs chan ReplyMessage, cfg config.Signaling) SignalingClient {
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
		sendMsgs:    sendMsgs,
		receiveSDPs: receiveSDPs,
		logger:      l,
		closeCtx:    ctx,
	}
	regCtx, cancel := context.WithTimeout(ctx, cfg.RegTimeout)
	defer cancel()
	if err := sc.registerConnect(regCtx, id); err != nil {
		fmt.Println(err)
		panic(fmt.Errorf("failed to register connect: %w", err))
	}
	go sc.sendMsg()
	go sc.receiveSDP()
	go sc.receiveResponses()
	return sc
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

func (sc *signalingClient) registerConnect(ctx context.Context, id string) error {
	var (
		op    = "signalingClient.registerConnect"
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
	regMsg := RegisterConnectMessage{
		ID: id,
	}
	dataReg, err := json.Marshal(regMsg)
	if err != nil {
		log.Error("failed to marshal register message", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	msg := Message{
		Id:   uuid.NewString(),
		Type: utils.Uint8ToPtr(REG_TYPE),
		Data: json.RawMessage(dataReg),
	}
	if err := encoder.Encode(msg); err != nil {
		log.Error("failed to encode register message to signaling", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}

	var responseMsg ResponseMessage
	if err := sc.decoder.Decode(&responseMsg); err != nil {
		log.Error("failed to decode response message from signaling", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	switch *responseMsg.Code {
	case SUCCESS:
		log.Info("connect registered successfully", idLog)
		return nil
	default:
		err := codeToError(op, *responseMsg.Code)
		log.Error("failed to register connect", idLog, logger.Err(err))
		return err
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
		var responseMsg ResponseMessage
		if err := sc.decoder.Decode(&responseMsg); err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to receive response message from signaling", logger.Err(err))
			continue
		}

		respLog := logger.NewLogData(logger.Attr("msgId", responseMsg.MessageId), logger.Attr("respCode", *responseMsg.Code))
		log.Info("new response message from signaling", respLog...)
		resChanInt, ok := sc.pendingResponses.Load(responseMsg.MessageId)
		if ok {
			// switch responseMsg.Code {
			// case SUCCESS:
			// 	reqChan, _ := resChanInt.(chan uint)
			// 	reqChan <- responseMsg.Code
			// case PAYLOAD_SUCCESS:
			reqChan, _ := resChanInt.(chan ResponseMessage)
			reqChan <- responseMsg
			// }

		}
	}
}

func (sc *signalingClient) sendMsg() {
	var (
		op  = "signalingClient.sendMsg"
		log = sc.logger.AddOp(op)
	)

	for msg := range sc.sendMsgs {
		select {
		case <-sc.closeCtx.Done():
			return
		default:
		}
		msgIdLog := logger.Attr("msgId", msg.Id)
		if err := sc.encoder.Encode(msg); err != nil {
			if cerr := utils.CheckErr(sc.closeCtx, err); cerr == nil {
				return
			}
			log.Error("failed to send msg to signaling", logger.Err(err), msgIdLog)
		} else {
			log.Info("msg sended successfully", msgIdLog)
		}
	}

}

func (sc *signalingClient) receiveSDP() {
	var (
		op  = "signalingClient.receiveSDP"
		log = sc.logger.AddOp(op)
	)

	for {
		select {
		case <-sc.closeCtx.Done():
			return
		default:
		}
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
		sdpMsg, err := ToReplyMessage(data)
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

	sendMsg := SendPayloadMessage{
		RecevierIDs: ids,
		Payload:     sdp,
	}
	sendData, err := json.Marshal(sendMsg)
	if err != nil {
		log.Error("failed to marshal sdp message", logger.Err(err))
		return errs.NewAppError(op, err)
	}
	msgId := uuid.NewString()
	respChan := make(chan uint, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(STREAM_TYPE),
		Data: sendData,
	}
	select {
	case sc.sendMsgs <- msg:
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	}
	select {
	case code := <-respChan:
		if code != SUCCESS {
			return codeToError(op, code)
		}
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	case <-sc.closeCtx.Done():
		return errs.AppClosing(op)
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
	msgId := uuid.NewString()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(GET_ONLINE_TYPE),
		Data: nil,
	}
	select {
	case sc.sendMsgs <- msg:
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	}
	onlineIds := []byte{}
	select {
	case respMsg := <-respChan:
		if *respMsg.Code != PAYLOAD_SUCCESS {
			return nil, codeToError(op, *respMsg.Code)
		}
		onlineIds = respMsg.Payload
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	case <-sc.closeCtx.Done():
		return nil, errs.AppClosing(op)
	}

	log.Info("online fetched from signaling")
	return onlineIds, nil
}

func codeToError(op string, code uint) error {
	switch code {
	case NOT_FOUND:
		return errs.ErrNotFound(op)
	case ALREADY_EXISTS:
		return errs.ErrAlreadyExists(op)
	case REQUEST_TIMEOUT:
		return errs.ErrRequestTimeout(op)
	default:
		return errs.ErrInternal(op)
	}
}

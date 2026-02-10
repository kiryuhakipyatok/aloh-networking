package client

import (
	"context"
	"encoding/json"
	"io"
	"networking/internal/utils"
	"networking/pkg/errs"
	"networking/pkg/logger"

	"github.com/google/uuid"
)

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
	regMsg := UserId{
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
	creds, err := ToCredsMessage(responseMsg.Payload)
	if err != nil {
		log.Error("failed to cast creds message", logger.Err(err), idLog)
	}
	sc.password = creds.Password
	sc.username = creds.Username
	return nil
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
			reqChan, _ := resChanInt.(chan ResponseMessage)
			reqChan <- responseMsg
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

func (sc *signalingClient) getPayload(ctx context.Context, msgType uint8, data []byte) ([]byte, error) {
	op := "signalingClient.getPayload"
	msgId := uuid.NewString()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(msgType),
		Data: data,
	}
	payload, err := sc.checkResp(ctx, checkConf{typee: PAYLOAD_SUCCESS, op: op, msg: msg, respChan: respChan})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (sc *signalingClient) sendPayload(ctx context.Context, ids []string, data []byte) error {
	op := "signalingClient.sendPayload"
	sendMsg := SendPayloadMessage{
		RecevierIDs: ids,
		Payload:     data,
	}
	sendData, err := json.Marshal(sendMsg)
	if err != nil {
		return errs.NewAppError(op, err)
	}
	msgId := uuid.NewString()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(STREAM_TYPE),
		Data: sendData,
	}
	_, err = sc.checkResp(ctx, checkConf{typee: SUCCESS, op: op, msg: msg, respChan: respChan})
	return err
}

func (sc *signalingClient) sendCommand(ctx context.Context, msgType uint8, data []byte) error {
	op := "signalingClient.sendCommand"
	msgId := uuid.NewString()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: utils.Uint8ToPtr(msgType),
		Data: data,
	}
	_, err := sc.checkResp(ctx, checkConf{typee: SUCCESS, op: op, msg: msg, respChan: respChan})
	return err
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

type checkConf struct {
	typee    uint
	op       string
	msg      Message
	respChan chan ResponseMessage
}

func (sc *signalingClient) checkResp(ctx context.Context, cc checkConf) ([]byte, error) {
	select {
	case sc.sendMsgs <- cc.msg:
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(cc.op)
	}
	select {
	case resp := <-cc.respChan:
		if *resp.Code != cc.typee {
			return nil, codeToError(cc.op, *resp.Code)
		}
		return resp.Payload, nil
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(cc.op)
	case <-sc.closeCtx.Done():
		return nil, errs.AppClosing(cc.op)
	}
}

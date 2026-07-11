package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"

	"github.com/cenkalti/backoff/v5"
	"github.com/kiryuhakipyatok/aloh-networking/internal/utils"
	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/logger"
	"github.com/quic-go/quic-go"

	"github.com/google/uuid"
)

func (sc *signalingClient) serveConnection(ctx context.Context, id uuid.UUID, b *backoff.ExponentialBackOff, cc connConf) error {
	op := "signalingClient.Run"
	log := sc.logger.AddOp(op)
	log.Info("serving connection")
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	conn, err := quic.DialAddr(connCtx, cc.addr, cc.tlsConf, cc.quicConf)
	if err != nil {
		log.Error("failed to dial quic conn", logger.Err(err))
		return errs.NewAppError(op, err)
	}

	sc.conn = conn

	if err := sc.registerConnect(connCtx, id); err != nil {
		log.Error("failed to register connect", logger.Err(err))
		if err := sc.conn.CloseWithError(0, "register failed"); err != nil {
			log.Error("failed to close failed conn", logger.Err(err))
		}
		sc.conn = nil
		return errs.NewAppError(op, err)
	}

	b.Reset()
	sc.isOnline.Store(true)

	errChan := make(chan error, 3)

	var wg sync.WaitGroup

	wg.Go(func() {
		if err := sc.sendMsg(connCtx); err != nil {
			errChan <- err
		}
	})

	wg.Go(func() {
		if err := sc.receiveSDP(connCtx); err != nil {
			errChan <- err
		}
	})

	wg.Go(func() {
		if err := sc.receiveResponses(connCtx); err != nil {
			errChan <- err
		}
	})

	var ferr error
	select {
	case <-ctx.Done():
		ferr = ctx.Err()
	case err := <-errChan:
		log.Error("error when running connection", logger.Err(err))
		ferr = errs.NewAppError(op, err)
	}

	cancel()
	sc.clearPendingResponses()
	sc.isOnline.Store(false)

	if sc.conn != nil && sc.ctrlStream != nil && !errors.Is(ferr, ctx.Err()) {
		if err := sc.CloseConnection(0, "conenction lost"); err != nil {
			log.Error("failed to close connection when connection lost", logger.Err(err))
		}
		sc.conn = nil
		sc.ctrlStream = nil
		sc.encoder = nil
		sc.decoder = nil
	}

	wg.Wait()
	return ferr
}

func (sc *signalingClient) registerConnect(ctx context.Context, id uuid.UUID) error {
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
	msgId := uuid.New()
	msg := Message{
		Id:   msgId,
		Type: new(uint8(REG_TYPE)),
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
	respCode := responseMsg.Code
	if respCode != PAYLOAD_SUCCESS {
		log.Error("response code is not payload success", idLog, logger.Attr("respCode", respCode))
		return codeToError(op, respCode)
	}

	creds, err := ToCredsMessage(responseMsg.Payload)
	if err != nil {
		log.Error("failed to cast creds message", logger.Err(err), idLog)
		return errs.NewAppError(op, err)
	}
	sc.password = creds.Password
	sc.username = creds.Username
	return nil

}

func (sc *signalingClient) receiveResponses(ctx context.Context) error {
	var (
		op  = "signalingClient.receiveResponses"
		log = sc.logger.AddOp(op)
	)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var responseMsg ResponseMessage
		if err := sc.decoder.Decode(&responseMsg); err != nil {
			if cerr := utils.CheckErr(ctx, err); cerr == nil {
				break
			}
			log.Error("failed to receive response message from signaling", logger.Err(err))
			return errs.NewAppError(op, err)
		}

		//respLog := logger.NewLogData(logger.Attr("msgId", responseMsg.MessageId), logger.Attr("respCode", *responseMsg.Code))
		//log.Info("new response message from signaling", respLog...)
		resChanInt, ok := sc.pendingResponses.Load(responseMsg.MessageId)
		if ok {
			reqChan, _ := resChanInt.(chan ResponseMessage)
			reqChan <- responseMsg
		}
	}

	return nil
}

func (sc *signalingClient) sendMsg(ctx context.Context) error {
	var (
		op  = "signalingClient.sendMsg"
		log = sc.logger.AddOp(op)
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-sc.sendMsgs:
			msgIdLog := logger.Attr("msgId", msg.Id)
			if err := sc.encoder.Encode(msg); err != nil {
				if cerr := utils.CheckErr(ctx, err); cerr == nil {
					break
				}
				log.Error("failed to send msg to signaling", logger.Err(err), msgIdLog)
				return errs.NewAppError(op, err)
			}
			// } else {
			// 	log.Info("msg sended successfully", msgIdLog)
			// }
		}
	}

}

func (sc *signalingClient) receiveSDP(ctx context.Context) error {
	var (
		op  = "signalingClient.receiveSDP"
		log = sc.logger.AddOp(op)
	)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		stream, err := sc.conn.AcceptUniStream(ctx)
		if err != nil {
			if cerr := utils.CheckErr(ctx, err); cerr == nil {
				return nil
			}
			log.Error("failed to accept uni stream from signaling", logger.Err(err))
			return errs.NewAppError(op, err)
		}
		streamIdLog := logger.Attr("streamId", stream.StreamID())
		data, err := io.ReadAll(stream)
		if err != nil {
			if cerr := utils.CheckErr(ctx, err); cerr == nil {
				return nil
			}
			log.Error("failed to read data from uni stream", logger.Err(err), streamIdLog)
			return errs.NewAppError(op, err)
		}
		sdpMsg, err := ToReplyMessage(data)
		if err != nil {
			if cerr := utils.CheckErr(ctx, err); cerr == nil {
				return nil
			}
			log.Error("failed cast sdp message to reply message", logger.Err(err), streamIdLog)
			return errs.NewAppError(op, err)
		}
		sc.receiveSDPs <- *sdpMsg
		log.Info("sdp received successfully", logger.Attr("senderId", sdpMsg.Sender))
	}

}

func (sc *signalingClient) getPayload(ctx context.Context, msgType uint8, data []byte) ([]byte, error) {
	op := "signalingClient.getPayload"
	if !sc.isOnline.Load() {
		return nil, errs.ErrOfflineBase
	}
	msgId := uuid.New()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: new(uint8(msgType)),
		Data: data,
	}
	payload, err := sc.checkResp(ctx, checkConf{typee: PAYLOAD_SUCCESS, op: op, msg: msg, respChan: respChan})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func (sc *signalingClient) sendPayload(ctx context.Context, ids []uuid.UUID, data []byte) error {
	op := "signalingClient.sendPayload"
	if !sc.isOnline.Load() {
		return errs.ErrOfflineBase
	}
	sendMsg := SendPayloadMessage{
		RecevierIDs: ids,
		Payload:     data,
	}
	sendData, err := json.Marshal(sendMsg)
	if err != nil {
		return errs.NewAppError(op, err)
	}
	msgId := uuid.New()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: new(uint8(STREAM_TYPE)),
		Data: sendData,
	}
	_, err = sc.checkResp(ctx, checkConf{typee: SUCCESS, op: op, msg: msg, respChan: respChan})
	return err
}

func (sc *signalingClient) clearPendingResponses() {
	sc.pendingResponses.Range(func(key, value any) bool {
		respChan := value.(chan ResponseMessage)
		close(respChan)
		sc.pendingResponses.Delete(key)
		return true
	})
}

func (sc *signalingClient) sendCommand(ctx context.Context, msgType uint8, data []byte) error {
	op := "signalingClient.sendCommand"
	if !sc.isOnline.Load() {
		return errs.ErrOfflineBase
	}
	msgId := uuid.New()
	respChan := make(chan ResponseMessage, 1)
	sc.pendingResponses.Store(msgId, respChan)
	defer sc.pendingResponses.Delete(msgId)
	msg := Message{
		Id:   msgId,
		Type: new(uint8(msgType)),
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
	case resp, ok := <-cc.respChan:
		if !ok {
			return nil, errs.ErrOfflineBase
		}
		if resp.Code != cc.typee {
			return nil, codeToError(cc.op, resp.Code)
		}
		return resp.Payload, nil
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(cc.op)
	case <-sc.closeCtx.Done():
		return nil, errs.AppClosing(cc.op)
	}
}

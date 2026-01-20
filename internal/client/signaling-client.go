package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"networking/internal/domain/repository"
	"networking/internal/protocol"
	"networking/internal/utils"
	"networking/pkg/errs"

	"github.com/google/uuid"
	"github.com/quic-go/quic-go"
)

type SignalingClient interface {
	Close(ctx context.Context, code uint, desc string)
	NewSDP(sdp []byte, ids []string) error
}

type signalingClient struct {
	Conn        *quic.Conn
	CtrlStream  *quic.Stream
	Decoder     *json.Decoder
	Encoder     *json.Encoder
	AgentRepo   repository.SessionRepository
	sendSDPs    chan protocol.Message
	receiveSDPs chan protocol.ReplyMessage
}

func NewSignalingClient(ar repository.SessionRepository, addr, id string, sendSDPs chan protocol.Message, receiveSDPs chan protocol.ReplyMessage) SignalingClient {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"aloh-proto"},
	}
	quicConf := &quic.Config{}
	conn, err := quic.DialAddr(context.Background(), addr, tlsConf, quicConf)
	if err != nil {
		panic(fmt.Errorf("failed to dial quic signaling: %w", err))
	}
	sc := &signalingClient{
		Conn:        conn,
		AgentRepo:   ar,
		sendSDPs:    sendSDPs,
		receiveSDPs: receiveSDPs,
	}
	if err := sc.registerConnect(context.Background(), id); err != nil {
		panic(err)
	}
	go sc.sendSDP()
	go sc.receiveSDP()
	go sc.receiveResponses()
	return sc
}

func (sc *signalingClient) Close(ctx context.Context, code uint, desc string) {
	if err := sc.CtrlStream.Close(); err != nil {
		panic(err)
	}
	if err := sc.Conn.CloseWithError(quic.ApplicationErrorCode(code), desc); err != nil {
		panic(err)
	}
}

func (sc *signalingClient) registerConnect(ctx context.Context, id string) error {
	op := "signalingClient.RegisterConnect"
	ctrlStream, err := sc.Conn.OpenStreamSync(ctx)
	if err != nil {
		return errs.NewAppError(op, err)
	}
	encoder := json.NewEncoder(ctrlStream)

	decoder := json.NewDecoder(ctrlStream)
	sc.Decoder = decoder
	sc.Encoder = encoder
	sc.CtrlStream = ctrlStream
	regMsg := protocol.RegisterConnectMessage{
		ID: id,
	}
	dataReg, err := json.Marshal(regMsg)

	if err != nil {
		return errs.NewAppError(op, err)
	}
	msg := protocol.Message{
		Id:   uuid.NewString(),
		Type: utils.Uint8ToPtr(0),
		Data: json.RawMessage(dataReg),
	}
	if err := encoder.Encode(msg); err != nil {
		return errs.NewAppError(op, err)
	}
	var responseMsg protocol.ResponseMessage
	if err := sc.Decoder.Decode(&responseMsg); err != nil {
		return errs.NewAppError(op, err)
	}
	fmt.Println(responseMsg)
	return nil
}

func (sc *signalingClient) receiveResponses() {
	for {
		var msg protocol.ResponseMessage
		if err := sc.Decoder.Decode(&msg); err != nil {
			panic(err)
		}
		fmt.Println("qwd", msg)
	}
}

func (sc *signalingClient) sendSDP() {
	for sdp := range sc.sendSDPs {
		if err := sc.Encoder.Encode(sdp); err != nil {
			panic(err)
		}
	}
}

func (sc *signalingClient) receiveSDP() {

	for {
		stream, err := sc.Conn.AcceptUniStream(context.Background())
		if err != nil {
			panic(err)
		}
		data, err := io.ReadAll(stream)
		if err != nil && !errors.Is(err, io.EOF) {
			panic(err)
		}
		// var msg protocol.Message
		// if err := sc.Decoder.Decode(&msg); err != nil {
		// 	if errors.Is(err, io.EOF) {
		// 		return
		// 	}
		// 	fmt.Println(err)
		// 	return
		// }
		fmt.Println(string(data))
		// var msg protocol.ReplyMessage
		// if err := json.Unmarshal(data, &msg); err != nil {
		// 	panic(err)
		// }
		// fmt.Println(msg)
		sdpMsg, err := protocol.ToReplyMessage(data)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			panic(err)
		}
		sc.receiveSDPs <- *sdpMsg
	}
}

func (sc *signalingClient) NewSDP(sdp []byte, ids []string) error {
	op := "signalingClient.sendSDP"
	sendMsg := protocol.SendPayloadMessage{
		RecevierIDs: ids,
		Payload:     sdp,
	}
	sendData, err := json.Marshal(sendMsg)
	if err != nil {
		return errs.NewAppError(op, err)
	}
	msg := protocol.Message{
		Id:   uuid.NewString(),
		Type: utils.Uint8ToPtr(1),
		Data: sendData,
	}
	sc.sendSDPs <- msg
	return nil
}

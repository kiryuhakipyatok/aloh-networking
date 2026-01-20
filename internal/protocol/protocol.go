package protocol

import (
	"encoding/json"
	"networking/pkg/errs"
)

type Message struct {
	Id   string          `json:"id" validate:"required,min=1"`
	Type *uint8          `json:"type" validate:"required"`
	Data json.RawMessage `json:"data" validate:"required"`
}

type RegisterConnectMessage struct {
	ID string `json:"id" validate:"required,min=1"`
}

type SendPayloadMessage struct {
	RecevierIDs []string `json:"ids" validate:"required,min=1"`
	Payload     []byte   `json:"payload" validate:"required"`
}

type DatagramProxingMessage struct {
	RecevierIDs []string `json:"ids" validate:"required,min=1"`
}

type ReplyMessage struct {
	Sender  string `json:"sender-id" validate:"required,min=1"`
	Payload []byte `json:"payload" validate:"required"`
}

type ResponseMessage struct {
	Code      int8   `json:"code"`
	MessageId string `json:"msgId"`
	Msg       string `json:"msg"`
}

func ToRegisterConnectMessage(data json.RawMessage) (*RegisterConnectMessage, error) {
	op := "protocols.ToRegisterConnectMessage"
	regMsg := &RegisterConnectMessage{}
	if err := json.Unmarshal(data, regMsg); err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}
	return regMsg, nil
}

func ToSendPayloadMessage(data json.RawMessage) (*SendPayloadMessage, error) {
	op := "protocols.ToSendPayloadMessage"
	connectMsg := &SendPayloadMessage{}
	if err := json.Unmarshal(data, connectMsg); err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}

	return connectMsg, nil
}

func ToReplyMessage(data []byte) (*ReplyMessage, error) {
	op := "protocols.ReplyMeToReplyMessagessage"
	replyMsg := &ReplyMessage{}
	if err := json.Unmarshal(data, replyMsg); err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}

	return replyMsg, nil
}

func NewReplyMessage(senderId string, pyaload json.RawMessage) ([]byte, error) {
	op := "protocols.NewReplyMessage"
	rm := ReplyMessage{
		Sender:  senderId,
		Payload: pyaload,
	}
	replyMsg, err := json.Marshal(rm)
	if err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}
	return replyMsg, nil
}

func ToDatagramProxingMessage(data json.RawMessage) (*DatagramProxingMessage, error) {
	op := "protocols.ToDatagramProxingMessage"
	datagramMsg := &DatagramProxingMessage{}
	if err := json.Unmarshal(data, datagramMsg); err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}
	return datagramMsg, nil
}

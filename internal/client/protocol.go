package client

import (
	"encoding/json"
	"github.com/kiryuhakipyatok/aloh-networking/pkg/errs/app"
)

const (
	REG_TYPE = iota
	STREAM_TYPE
	DATAGRAM_TYPE
	DISCONN_TYPE
	GET_ONLINE_TYPE
	ADD_IN_SESSION
	GET_SESSIONS_BY_ID
	DELETE_FROM_SESSION
)

const (
	SUCCESS = iota
	NOT_FOUND
	ALREADY_EXISTS
	REQUEST_TIMEOUT
	PAYLOAD_SUCCESS
	INVALID_PROTOCOL
	STREAM_ERROR
	INVALID_TYPE
	INTERNAL_ERROR
)

type Message struct {
	Id   string          `json:"id" validate:"required,min=1"`
	Type *uint8          `json:"type" validate:"required"`
	Data json.RawMessage `json:"data" validate:"required"`
}

type UserId struct {
	ID string `json:"id" validate:"required,min=1"`
}

type SendPayloadMessage struct {
	RecevierIDs []string `json:"ids" validate:"required,min=1"`
	Payload     []byte   `json:"payload" validate:"required"`
}

type ReplyMessage struct {
	Sender  string `json:"sender-id" validate:"required,min=1"`
	Payload []byte `json:"payload" validate:"required"`
}

type ResponseMessage struct {
	Code      *uint           `json:"code"`
	MessageId string          `json:"msgId"`
	Payload   json.RawMessage `json:"payload"`
}

type CredsMessage struct {
	Username string `json:"username" validate:"required,min=1"`
	Password string `json:"password" validate:"required,min=1"`
}

func ToRegisterConnectMessage(data json.RawMessage) (*UserId, error) {
	op := "protocols.ToRegisterConnectMessage"
	regMsg := &UserId{}
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

func ToCredsMessage(data json.RawMessage) (*CredsMessage, error) {
	op := "protocols.ToCredsMessage"
	creds := &CredsMessage{}
	if err := json.Unmarshal(data, creds); err != nil {
		return nil, errs.ErrInvalidJson(op, err)
	}

	return creds, nil
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

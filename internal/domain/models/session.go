package models

import (
	"encoding/json"
	"sync"

	"github.com/pion/ice/v2"
	"github.com/quic-go/quic-go"
)

type Session struct {
	UserID          string
	CurrentConnects []string
	Agent           *ice.Agent
	Conn            *quic.Conn
	EventStream     *quic.Stream
	EventDecoder    *json.Decoder
	EventEncoder    *json.Encoder
	IsInitiator     bool
	CredsChan       chan struct{}
	Closing         sync.Once
	ReadyChan       chan struct{}
	Key             []byte
}

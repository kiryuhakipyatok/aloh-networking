package models

import (
	"sync"

	"github.com/pion/ice/v2"
	"github.com/quic-go/quic-go"
)

type Session struct {
	UserID      string
	Agent       *ice.Agent
	Conn        *quic.Conn
	IsInitiator bool
	CredsChan   chan struct{}
	Closing     sync.Once
	ReadyChan   chan struct{}
	Key         []byte
}

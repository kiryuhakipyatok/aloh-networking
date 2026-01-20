package models

import (
	"github.com/pion/ice/v2"
	"github.com/quic-go/quic-go"
)

type Session struct {
	UserID      string
	Agent       *ice.Agent
	Conn        *quic.Conn
	IsInitiator bool
	State       string
	CredsChan   chan struct{}
}

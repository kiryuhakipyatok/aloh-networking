package p2p

import (
	"sync"

	"github.com/pion/ice/v2"
	"github.com/quic-go/quic-go"
)

type Peer struct {
	UserID      string
	Agent       *ice.Agent
	Conn        *quic.Conn
	IsInitiator bool
	CredsChan   chan struct{}
	Closing     sync.Once
}

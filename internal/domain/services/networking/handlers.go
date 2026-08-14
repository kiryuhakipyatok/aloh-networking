package networking

import (
	"sync/atomic"

	"github.com/google/uuid"
)

type dataHandler func(id uuid.UUID, data []byte)
type connectionHandler func(id uuid.UUID)
type eventHandler func(id uuid.UUID, data Event)

type handlers struct {
	onChatHandler             atomic.Value
	onVoiceHandler            atomic.Value
	onWebcamHandler           atomic.Value
	onScreenHandler           atomic.Value
	onPeerConnectedHandler    atomic.Value
	onPeerDisconnectedHandler atomic.Value
	onEventHandler            atomic.Value
	onOnlineFriendHandler     atomic.Value
}

func (ns *networkingServ) SaveChatHandler(h dataHandler) {
	ns.onChatHandler.Store(h)
}

func (ns *networkingServ) SaveVoiceHandler(h dataHandler) {
	ns.onVoiceHandler.Store(h)
}

func (ns *networkingServ) SaveWebcamHandler(h dataHandler) {
	ns.onWebcamHandler.Store(h)
}

func (ns *networkingServ) SaveScreenHandler(h dataHandler) {
	ns.onScreenHandler.Store(h)
}

func (ns *networkingServ) SavePeerConnectedHandler(h connectionHandler) {
	ns.onPeerConnectedHandler.Store(h)
}

func (ns *networkingServ) SavePeerDisconnectedHandler(h connectionHandler) {
	ns.onPeerDisconnectedHandler.Store(h)
}

func (ns *networkingServ) SaveEventHandler(h eventHandler) {
	ns.onEventHandler.Store(h)
}

func (ns *networkingServ) SaveOnlineFriendHandler(h connectionHandler) {
	ns.onOnlineFriendHandler.Store(h)
}

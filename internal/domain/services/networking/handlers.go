package networking

import "sync/atomic"

type dataHandler func(id string, data []byte)
type connectionHandler func(id string)
type eventHandler func(id string, data Event)

type handlers struct {
	onChatHandler             atomic.Value
	onVoiceHandler            atomic.Value
	onVideoHandler            atomic.Value
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

func (ns *networkingServ) SaveVideoHandler(h dataHandler) {
	ns.onVideoHandler.Store(h)
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
package networking

import "sync/atomic"

type handler func(data []byte)

type handlers struct {
	onChatHandler  atomic.Value
	onVoiceHandler atomic.Value
	onVideoHandler atomic.Value
}

func (ns *networkingServ) SaveChatHandler(h handler) {
	ns.onChatHandler.Store(h)
}

func (ns *networkingServ) SaveVoiceHandler(h handler) {
	ns.onVoiceHandler.Store(h)
}

func (ns *networkingServ) SaveVideoHandler(h handler) {
	ns.onVideoHandler.Store(h)
}

package alohnetwork

import "github.com/google/uuid"

func (n *Netwoking) RegisterOnChat(cb func(id uuid.UUID, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnChat(func(id uuid.UUID, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnVideo(cb func(id uuid.UUID, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnVideo(func(id uuid.UUID, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnVoice(cb func(id uuid.UUID, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnVoice(func(id uuid.UUID, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnPeerConnected(cb func(id uuid.UUID)) {
	if cb == nil {
		return
	}
	n.Handler.OnPeerConnected(func(id uuid.UUID) {
		cb(id)
	})
}

func (n *Netwoking) RegisterOnPeerDisconnected(cb func(id uuid.UUID)) {
	if cb == nil {
		return
	}
	n.Handler.OnPeerDisconnected(func(id uuid.UUID) {
		cb(id)
	})
}

func (n *Netwoking) RegisterOnEvent(cb func(id uuid.UUID, e Event)) {
	if cb == nil {
		return
	}
	n.Handler.OnEvent(func(id uuid.UUID, e Event) {
		cb(id, e)
	})
}

package alohnetwork

func (n *Netwoking) RegisterOnChat(cb func(id string, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnChat(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnVideo(cb func(id string, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnVideo(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnVoice(cb func(id string, data []byte)) {
	if cb == nil {
		return
	}
	n.Handler.OnVoice(func(id string, data []byte) {
		if len(data) == 0 {
			return
		}
		cb(id, data)
	})
}

func (n *Netwoking) RegisterOnPeerConnected(cb func(id string)) {
	if cb == nil {
		return
	}
	n.Handler.OnPeerConnected(func(id string) {
		cb(id)
	})
}

func (n *Netwoking) RegisterOnPeerDisconnected(cb func(id string)) {
	if cb == nil {
		return
	}
	n.Handler.OnPeerDisconnected(func(id string) {
		cb(id)
	})
}

func (n *Netwoking) RegisterOnEvent(cb func(id string, e Event)) {
	if cb == nil {
		return
	}
	n.Handler.OnEvent(func(id string, e Event) {
		cb(id, e)
	})
}

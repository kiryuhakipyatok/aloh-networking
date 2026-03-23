package api

import (
	"context"

	"github.com/kiryuhakipyatok/aloh-networking/internal/app"
	"github.com/kiryuhakipyatok/aloh-networking/internal/handlers"
)

type Netwoking struct {
	Handler *handlers.NetworkingHandler
	Cancel  context.CancelFunc
}

func NewNetworking(userId, logPath string) Netwoking {
	service, cancel, cfg := app.Init(userId, logPath)
	nh := handlers.NewNetworkingHandler(service, cfg)
	return Netwoking{
		Handler: nh,
		Cancel:  cancel,
	}
}

func (n *Netwoking) Delete() {
	if n.Cancel != nil {
		n.Cancel()
	}
}

func (n *Netwoking) Connect(id string) error {
	if err := n.Handler.Connect(id); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) Disconnect() error {
	if err := n.Handler.Disconnect(); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) SendMessage(msg string) error {
	if err := n.Handler.SendMessage(msg); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) SendVoice(data []byte) error {
	if err := n.Handler.SendVoice(data); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) SendVideo(data []byte) error {
	if err := n.Handler.SendVideo(data); err != nil {
		return err
	}
	return nil
}

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

func (n *Netwoking) FetchOnline() ([]string, error) {
	online, err := n.Handler.FetchOnline()
	if err != nil {
		return nil, err
	}
	return online, nil
}

func (n *Netwoking) FetchSessions(id string) ([]string, error) {
	sessions, err := n.Handler.FetchSessionById(id)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}
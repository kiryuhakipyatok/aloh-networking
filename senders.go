package alohnetwork

import "github.com/kiryuhakipyatok/aloh-networking/internal/domain/services/networking"

func (n *Netwoking) SendEvent(e networking.Event) error {
	if err := n.Handler.SendEvent(e); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) SendMessage(msg []byte) error {
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

func (n *Netwoking) SendWebcam(data []byte) error {
	if err := n.Handler.SendWebcam(data); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) SendScreen(data []byte) error {
	if err := n.Handler.SendScreen(data); err != nil {
		return err
	}
	return nil
}
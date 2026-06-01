package alohnetwork

func (n *Netwoking) Connect(id string) error {
	if err := n.Handler.Connect(id); err != nil {
		return err
	}
	return nil
}

func (n *Netwoking) ConnectById(id string) error {
	if err := n.Handler.ConnectById(id); err != nil {
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

func (n *Netwoking) DisconnectById(id string) error {
	if err := n.Handler.DisconnectById(id); err != nil {
		return err
	}
	return nil
}

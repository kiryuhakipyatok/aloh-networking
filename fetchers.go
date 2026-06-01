package alohnetwork

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

func (n *Netwoking) FetchFriends(ids []string) (map[string][]string, error) {
	friends, err := n.Handler.FetchOnlineFriends(ids)
	if err != nil {
		return nil, err
	}
	return friends, nil
}
package alohnetwork

import "github.com/google/uuid"

func (n *Netwoking) FetchOnline() ([]uuid.UUID, error) {
	online, err := n.Handler.FetchOnline()
	if err != nil {
		return nil, err
	}
	return online, nil
}

func (n *Netwoking) FetchSessions(id uuid.UUID) ([]uuid.UUID, error) {
	sessions, err := n.Handler.FetchSessionById(id)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (n *Netwoking) FetchFriends(ids []uuid.UUID) (map[uuid.UUID][]string, error) {
	friends, err := n.Handler.FetchOnlineFriends(ids)
	if err != nil {
		return nil, err
	}
	return friends, nil
}

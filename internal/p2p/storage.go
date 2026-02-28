package p2p

import (
	"context"
	"errors"
	"networking/pkg/errs"
	"sync"
)

type PeerStorage interface {
	Add(ctx context.Context, id string, peer *Peer) error
	Delete(ctx context.Context, id string) error
	Fetch(ctx context.Context) ([]*Peer, error)
	Get(ctx context.Context, id string) (*Peer, error)
	Clear(ctx context.Context) error
}

type peerStorage struct {
	sync.Map
}

func NewPeerStorage() PeerStorage {
	return &peerStorage{}
}

func (pr *peerStorage) Add(ctx context.Context, id string, peer *Peer) error {
	op := "peerStorage.Add"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		if _, ok := pr.LoadOrStore(id, peer); ok {
			return errs.ErrAlreadyExists(op)
		}
		return nil
	}
}

func (pr *peerStorage) Delete(ctx context.Context, id string) error {
	op := "peerStorage.Delete"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		if _, ok := pr.LoadAndDelete(id); !ok {
			return errs.ErrNotFound(op)
		}
		return nil
	}
}

func (pr *peerStorage) Get(ctx context.Context, id string) (*Peer, error) {
	op := "peerStorage.Get"
	select {
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
		val, ok := pr.Load(id)
		if !ok {
			return nil, errs.ErrNotFound(op)
		}
		user, ok := val.(*Peer)
		if !ok {
			return nil, errors.New("invalid value type")
		}
		return user, nil
	}
}

func (pr *peerStorage) Fetch(ctx context.Context) ([]*Peer, error) {
	op := "peerStorage.Fetch"
	sessions := []*Peer{}
	select {
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
		pr.Range(func(key, value any) bool {
			peer, ok := value.(*Peer)
			if !ok {
				return false
			}
			sessions = append(sessions, peer)
			return true
		})
	}
	return sessions, nil
}

func (pr *peerStorage) Clear(ctx context.Context) error {
	op := "peerStorage.Clear"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		pr.Range(func(key, value any) bool {
			if _, ok := pr.LoadAndDelete(key); !ok {
				return false
			}
			return true
		})
	}
	return nil
}

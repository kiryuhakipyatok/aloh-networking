package repository

import (
	"context"
	"errors"
	"networking/internal/domain/models"
	"networking/pkg/errs"
	"sync"
)

type SessionRepository interface {
	Add(ctx context.Context, id string, session *models.Session) error
	Delete(ctx context.Context, id string) error
	Fetch(ctx context.Context) ([]*models.Session, error)
	Get(ctx context.Context, id string) (*models.Session, error)
	Clear(ctx context.Context) error
}

type sessionRepository struct {
	sync.Map
}

func NewSessionRepository() SessionRepository {
	return &sessionRepository{}
}

func (ag *sessionRepository) Add(ctx context.Context, id string, session *models.Session) error {
	op := "sessionRepository.Add"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		if _, ok := ag.LoadOrStore(id, session); ok {
			return errs.ErrAlreadyExists(op)
		}
		return nil
	}
}

func (ag *sessionRepository) Delete(ctx context.Context, id string) error {
	op := "sessionRepository.Delete"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		if _, ok := ag.LoadAndDelete(id); !ok {
			return errs.ErrNotFound(op)
		}
		return nil
	}
}

func (ag *sessionRepository) Get(ctx context.Context, id string) (*models.Session, error) {
	op := "sessionRepository.Get"
	select {
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
		val, ok := ag.Load(id)
		if !ok {
			return nil, errs.ErrNotFound(op)
		}
		user, ok := val.(*models.Session)
		if !ok {
			return nil, errors.New("invalid value type")
		}
		return user, nil
	}
}

func (ag *sessionRepository) Fetch(ctx context.Context) ([]*models.Session, error) {
	op := "sessionRepository.Fetch"
	sessions := []*models.Session{}
	select {
	case <-ctx.Done():
		return nil, errs.ErrRequestTimeout(op)
	default:
		ag.Range(func(key, value any) bool {
			session, ok := value.(*models.Session)
			if !ok {
				return false
			}
			sessions = append(sessions, session)
			return true
		})
	}
	return sessions, nil
}

func (ag *sessionRepository) Clear(ctx context.Context) error {
	op := "sessionRepository.Clear"
	select {
	case <-ctx.Done():
		return errs.ErrRequestTimeout(op)
	default:
		ag.Range(func(key, value any) bool {
			if _, ok := ag.LoadAndDelete(key); !ok {
				return false
			}
			return true
		})
	}
	return nil
}

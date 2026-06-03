package alohnetwork

import (
	"context"

	"github.com/kiryuhakipyatok/aloh-networking/config"
	"github.com/kiryuhakipyatok/aloh-networking/internal/app"
	"github.com/kiryuhakipyatok/aloh-networking/internal/handlers"
	errs "github.com/kiryuhakipyatok/aloh-networking/pkg/errs/handlers"
)

type Netwoking struct {
	Handler *handlers.NetworkingHandler
	Cancel  context.CancelFunc
}

func NewNetworking(userId string, friends []string, cfg config.Config) (*Netwoking, error) {
	service, cancel, err := app.Init(userId, friends, cfg)
	if err != nil {
		return nil, errs.ProcessError(err)
	}
	nh := handlers.NewNetworkingHandler(service, cfg.Handler)
	return &Netwoking{
		Handler: nh,
		Cancel:  cancel,
	}, nil
}

func (n *Netwoking) Delete() {
	if n.Cancel != nil {
		n.Cancel()
	}
}

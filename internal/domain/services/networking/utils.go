package networking

import (
	"context"
	"io"

	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/e2ee"
	"github.com/kiryuhakipyatok/aloh-networking/internal/domain/models"
)

func setupE2EE(ctx context.Context, session *models.Session) error {
	privKey, publicKey, err := e2ee.NewKeys()
	if err != nil {
		return err
	}
	stream, err := session.Conn.OpenUniStreamSync(ctx)
	if err != nil {
		return err
	}
	if _, err := stream.Write(publicKey.Bytes()); err != nil {
		return err
	}
	if err := stream.Close(); err != nil {
		return err
	}
	recStream, err := session.Conn.AcceptUniStream(ctx)
	if err != nil {
		return err
	}
	recPublicKeyBytes, err := io.ReadAll(recStream)
	if err != nil {
		return err
	}
	recStream.CancelRead(0)
	key, err := e2ee.NewMasterKey(privKey, recPublicKeyBytes)
	if err != nil {
		return err
	}
	session.Key = key
	return nil
}

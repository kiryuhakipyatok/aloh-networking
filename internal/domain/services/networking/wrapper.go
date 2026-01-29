package networking

import (
	"net"
	"time"

	"github.com/pion/ice/v2"
)

type PacketConnWrapper struct {
	Conn      *ice.Conn
	FixedAddr net.Addr
}

func (c *PacketConnWrapper) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Conn.Read(p)
	return n, c.FixedAddr, err
}

func (c *PacketConnWrapper) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Conn.Write(p)
}

func (c *PacketConnWrapper) Close() error {
	return c.Conn.Close()
}

func (c *PacketConnWrapper) LocalAddr() net.Addr {
	return c.Conn.LocalAddr()
}

func (c *PacketConnWrapper) SetDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *PacketConnWrapper) SetReadDeadline(t time.Time) error {
	return c.Conn.SetDeadline(t)
}

func (c *PacketConnWrapper) SetWriteDeadline(t time.Time) error {
	return c.Conn.SetWriteDeadline(t)
}

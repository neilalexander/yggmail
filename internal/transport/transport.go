package transport

import (
	"net"
)

type Transport interface {
	Dial(host string) (net.Conn, error)
	Listener() net.Listener
}

package tcp

import (
	"context"
	"log"
	"net"
	"syscall"

	"golang.org/x/sys/windows"
)

const TCP_FASTOPEN = 15

func setTFO(fd uintptr) {
	if err := windows.SetsockoptInt(windows.Handle(fd), windows.IPPROTO_TCP, TCP_FASTOPEN, 1); err != nil {
		log.Printf("[warning] Failed to set socket option TCP_FASTOPEN: %v", err)
	}
}

func ListenTCP(address string, tfo bool) (net.Listener, error) {
	var lc net.ListenConfig
	if tfo {
		lc.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(setTFO)
		}
	}
	return lc.Listen(context.Background(), "tcp", address)
}

func DialTCP(address string, tfo bool) (net.Conn, error) {
	var d net.Dialer
	if tfo {
		d.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(setTFO)
		}
	}
	return d.Dial("tcp", address)
}

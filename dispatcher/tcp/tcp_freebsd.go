package tcp

import (
	"context"
	"log"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func ListenTCP(address string, tfo bool) (net.Listener, error) {
	var lc net.ListenConfig
	if tfo {
		lc.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1); err != nil {
					log.Printf("[warning] Failed to set socket option TCP_FASTOPEN for listener: %v", err)
				}
			})
		}
	}
	return lc.Listen(context.Background(), "tcp", address)
}

func DialTCP(address string, tfo bool) (net.Conn, error) {
	var d net.Dialer
	if tfo {
		d.Control = func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				if err := unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_FASTOPEN, 1); err != nil {
					log.Printf("[warning] Failed to set socket option TCP_FASTOPEN for dialer: %v", err)
				}
			})
		}
	}
	return d.Dial("tcp", address)
}

//go:build !darwin && !freebsd && !linux && !windows
// +build !darwin,!freebsd,!linux,!windows

package tcp

import (
	"log"
	"net"
)

func ListenTCP(address string, tfo bool) (net.Listener, error) {
	if tfo {
		log.Printf("[warning] mmp-go does not support setting TCP fast open socket option on this platform.\n")
	}
	return net.Listen("tcp", address)
}

func DialTCP(address string, tfo bool) (net.Conn, error) {
	if tfo {
		log.Printf("[warning] mmp-go does not support setting TCP fast open socket option on this platform.\n")
	}
	return net.Dial("tcp", address)
}

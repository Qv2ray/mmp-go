package infra

import "errors"

var ErrNetClosing = errors.New("use of closed network connection")


func AddrLen(packet []byte) int {
	if len(packet) < 5 {
		return 0 // invalid addr field
	}
	l := 1 + 2 // type + port
	// host
	switch packet[0] {
	case 0x01:
		l += 4
	case 0x03:
		l += 1 + int(packet[1])
	case 0x04:
		l += 16
	}
	return l
}
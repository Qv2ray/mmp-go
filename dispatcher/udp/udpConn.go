package udp

import (
	"net"
	"sync"
	"time"
)

const (
	defaultNatTimeout = 2 * time.Minute
	dnsQueryTimeout   = 17 * time.Second // RFC 5452
)

type UDPConn struct {
	Establishing chan struct{}
	*net.UDPConn
}

func NewUDPConn(conn *net.UDPConn) *UDPConn {
	c := &UDPConn{
		UDPConn:      conn,
		Establishing: make(chan struct{}),
	}
	if c.UDPConn != nil {
		close(c.Establishing)
	}
	return c
}

type UDPConnMapping struct {
	nm map[string]*UDPConn
	sync.Mutex
}

func NewUDPConnMapping() *UDPConnMapping {
	m := &UDPConnMapping{
		nm: make(map[string]*UDPConn),
	}
	return m
}

func (m *UDPConnMapping) Get(key string) (conn *UDPConn, ok bool) {
	v, ok := m.nm[key]
	if ok {
		conn = v
	}
	return
}

// pass val=nil for stating it is establishing
func (m *UDPConnMapping) Insert(key string, val *net.UDPConn) {
	m.nm[key] = NewUDPConn(val)
}

func (m *UDPConnMapping) Remove(key string) {
	v, ok := m.nm[key]
	if !ok {
		return
	}
	select {
	case <-v.Establishing:
		_ = v.Close()
	default:
		close(v.Establishing)
	}
	delete(m.nm, key)
}

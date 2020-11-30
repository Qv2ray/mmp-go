package udp

import (
	"net"
	"sync"
	"time"
)

const timeout = 120 * time.Second

type UDPConn struct {
	*net.UDPConn
	lastVisitTime time.Time
}

func NewUDPConn(conn *net.UDPConn) *UDPConn {
	return &UDPConn{
		UDPConn:       conn,
		lastVisitTime: time.Now(),
	}
}

// TODO: clean in time
type UDPConnMapping struct {
	nm map[string]*UDPConn
	sync.Mutex
}

func NewUDPConnMapping() *UDPConnMapping {
	return &UDPConnMapping{
		nm: make(map[string]*UDPConn),
	}
}

func (m *UDPConnMapping) Get(key string) (conn *net.UDPConn, ok bool) {
	v, ok := m.nm[key]
	if ok {
		if time.Since(v.lastVisitTime) > timeout {
			return nil, false
		}
		v.lastVisitTime = time.Now()
		conn = v.UDPConn
	}
	return
}

func (m *UDPConnMapping) Insert(key string, val *net.UDPConn) {
	m.nm[key] = NewUDPConn(val)
}

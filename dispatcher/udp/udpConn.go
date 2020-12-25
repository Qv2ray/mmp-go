package udp

import (
	"net"
	"sync"
	"time"
)

const (
	defaultTimeout  = 120 * time.Second
	dnsQueryTimeout = 17 * time.Second
)

type UDPConn struct {
	Establishing chan struct{}
	*net.UDPConn
	lastVisitTime time.Time
	timeout       time.Duration
}

func NewUDPConn(conn *net.UDPConn, timeout time.Duration) *UDPConn {
	c := &UDPConn{
		UDPConn:       conn,
		lastVisitTime: time.Now(),
		timeout:       timeout,
		Establishing:  make(chan struct{}),
	}
	if c.UDPConn != nil {
		close(c.Establishing)
	}
	return c
}

type UDPConnMapping struct {
	nm map[string]*UDPConn
	sync.Mutex
	cleanTicker *time.Ticker
}

func (m *UDPConnMapping) cleaner() {
	for t := range m.cleanTicker.C {
		m.Lock()
		for k, v := range m.nm {
			if t.Sub(v.lastVisitTime) > v.timeout {
				delete(m.nm, k)
			}
		}
		m.Unlock()
	}
}

func NewUDPConnMapping() *UDPConnMapping {
	m := &UDPConnMapping{
		nm:          make(map[string]*UDPConn),
		cleanTicker: time.NewTicker(2 * time.Second),
	}
	go m.cleaner()
	return m
}

func (m *UDPConnMapping) Close() error {
	m.cleanTicker.Stop()
	return nil
}

func (m *UDPConnMapping) Get(key string) (conn *UDPConn, ok bool) {
	v, ok := m.nm[key]
	if ok {
		if time.Since(v.lastVisitTime) > defaultTimeout {
			return nil, false
		}
		v.lastVisitTime = time.Now()
		conn = v
	}
	return
}

// pass val=nil for stating it is establishing
func (m *UDPConnMapping) Insert(key string, val *net.UDPConn, timeout time.Duration) {
	m.nm[key] = NewUDPConn(val, timeout)
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

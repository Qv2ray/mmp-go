package tcp

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"github.com/Qv2ray/mmp-go/infra/pool"
)

//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
const (
	BasicLen = 32 + 2 + 16
	MaxLen   = BasicLen + 16383 + 16
)

func init() {
	dispatcher.Register("tcp", New)
}

// DuplexConn is a net.Conn that allows for closing only the reader or writer end of
// it, supporting half-open state.
type DuplexConn interface {
	net.Conn
	// Closes the Read end of the connection, allowing for the release of resources.
	// No more reads should happen.
	CloseRead() error
	// Closes the Write end of the connection. An EOF or FIN signal may be
	// sent to the connection target.
	CloseWrite() error
}

type TCP struct {
	gMutex sync.RWMutex
	group  *config.Group
	l      net.Listener
}

func New(g *config.Group) (d dispatcher.Dispatcher) {
	return &TCP{group: g}
}

func (d *TCP) Listen() (err error) {
	d.l, err = ListenTCP(fmt.Sprintf(":%d", d.group.Port), d.group.ListenerTCPFastOpen)
	if err != nil {
		return
	}
	defer d.l.Close()
	log.Printf("[tcp] listen on :%v\n", d.group.Port)
	for {
		conn, err := d.l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		go func() {
			err := d.handleConn(conn)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

func (d *TCP) UpdateGroup(group *config.Group) {
	d.gMutex.Lock()
	defer d.gMutex.Unlock()
	d.group = group
}

func (d *TCP) Close() (err error) {
	log.Printf("[tcp] closed :%v\n", d.group.Port)
	return d.l.Close()
}

func (d *TCP) handleConn(conn net.Conn) error {
	/*
	   https://github.com/shadowsocks/shadowsocks-org/blob/master/whitepaper/whitepaper.md
	*/
	defer conn.Close()

	if d.group.AuthTimeoutSec > 0 {
		conn.SetReadDeadline(time.Now().Add(time.Duration(d.group.AuthTimeoutSec) * time.Second))
	}

	data := pool.Get(MaxLen)
	defer pool.Put(data)
	buf := pool.Get(BasicLen)
	defer pool.Put(buf)
	n, err := io.ReadAtLeast(conn, data, BasicLen)
	if err != nil {
		return fmt.Errorf("[tcp] %s <-x-> %s handleConn ReadAtLeast error: %w", conn.RemoteAddr(), conn.LocalAddr(), err)
	}

	// get user's context (preference)
	d.gMutex.RLock() // avoid insert old servers to the new userContextPool
	userContext := d.group.UserContextPool.GetOrInsert(conn.RemoteAddr(), d.group.Servers)
	d.gMutex.RUnlock()

	// auth every server
	server, _ := d.Auth(buf, data, userContext)
	if server == nil {
		if len(d.group.Servers) == 0 {
			return nil
		}
		// fallback
		server = &d.group.Servers[0]
	}

	if d.group.AuthTimeoutSec > 0 {
		conn.SetReadDeadline(time.Time{})
	}

	// dial and relay
	rc, err := DialTCP(server.Target, server.TCPFastOpen)
	if err != nil {
		return fmt.Errorf("[tcp] %s <-> %s <-x-> %s handleConn dial error: %w", conn.RemoteAddr(), conn.LocalAddr(), server.Target, err)
	}

	_, err = rc.Write(data[:n])
	if err != nil {
		return fmt.Errorf("[tcp] %s <-> %s <-x-> %s handleConn write error: %w", conn.RemoteAddr(), conn.LocalAddr(), server.Target, err)
	}

	log.Printf("[tcp] %s <-> %s <-> %s", conn.RemoteAddr(), conn.LocalAddr(), server.Target)

	if err := relay(conn.(DuplexConn), rc.(DuplexConn)); err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return nil // ignore i/o timeout
		}
		return fmt.Errorf("[tcp] handleConn relay error: %w", err)
	}
	return nil
}

func relay(lc, rc DuplexConn) error {
	defer rc.Close()
	ch := make(chan error, 1)
	go func() {
		_, err := io.Copy(lc, rc)
		rc.CloseRead()
		lc.CloseWrite()
		ch <- err
	}()
	_, err := io.Copy(rc, lc)
	lc.CloseRead()
	rc.CloseWrite()
	innerErr := <-ch
	if err != nil {
		return err
	}
	return innerErr
}

func (d *TCP) Auth(buf []byte, data []byte, userContext *config.UserContext) (hit *config.Server, content []byte) {
	if len(data) < BasicLen {
		return nil, nil
	}
	return userContext.Auth(func(server *config.Server) ([]byte, bool) {
		return probe(buf, data, server)
	})
}

func probe(buf []byte, data []byte, server *config.Server) ([]byte, bool) {
	//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
	conf := cipher.CiphersConf[server.Method]

	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen : conf.SaltLen+2+conf.TagLen]

	return conf.Verify(buf, server.MasterKey, salt, cipherText, nil)
}

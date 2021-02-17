package tcp

import (
	"errors"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"github.com/Qv2ray/mmp-go/infra/pool"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
const (
	DefaultTimeout = 7440 * time.Second
	BasicLen       = 32 + 2 + 16
)

func init() {
	dispatcher.Register("tcp", New)
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
	d.l, err = net.Listen("tcp", fmt.Sprintf(":%d", d.group.Port))
	if err != nil {
		return
	}
	defer d.l.Close()
	log.Printf("[tcp] listen on :%v\n", d.group.Port)
	for {
		conn, err := d.l.Accept()
		if err != nil {
			switch err := err.(type) {
			case *net.OpError:
				if errors.Is(err.Unwrap(), net.ErrClosed) {
					return nil
				}
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
	var (
		server      *config.Server
		userContext *config.UserContext
	)
	defer conn.Close()

	var data = pool.Get(BasicLen)
	defer pool.Put(data)
	var buf = pool.Get(BasicLen)
	defer pool.Put(buf)
	n, err := io.ReadFull(conn, data)
	if err != nil {
		return fmt.Errorf("[tcp] handleConn readfull error: %v", err)
	}

	// get user's context (preference)
	d.gMutex.RLock() // avoid insert old servers to the new userContextPool
	userContext = d.group.UserContextPool.GetOrInsert(conn.RemoteAddr(), d.group.Servers)
	d.gMutex.RUnlock()

	// auth every server
	server, _ = d.Auth(buf, data, userContext)
	if server == nil {
		if len(d.group.Servers) == 0 {
			return nil
		}
		// fallback
		server = &d.group.Servers[0]
	}

	// dial and relay
	rc, err := net.Dial("tcp", server.Target)
	if err != nil {
		return fmt.Errorf("[tcp] handleConn dial error: %v", err)
	}

	_ = rc.SetDeadline(time.Now().Add(DefaultTimeout))
	_, err = rc.Write(data[:n])
	if err != nil {
		return fmt.Errorf("[tcp] handleConn write error: %v", err)
	}

	log.Printf("[tcp] %s <-> %s <-> %s", conn.RemoteAddr(), conn.LocalAddr(), rc.RemoteAddr())

	if err := relay(conn, rc); err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return nil // ignore i/o timeout
		}
		return fmt.Errorf("[tcp] handleConn relay error: %v", err)
	}
	return nil
}

func relay(lc, rc net.Conn) error {
	defer rc.Close()
	ch := make(chan error, 1)
	go func() {
		_, err := io.Copy(lc, rc)
		lc.SetDeadline(time.Now())
		rc.SetDeadline(time.Now())
		ch <- err
	}()
	_, err := io.Copy(rc, lc)
	lc.SetDeadline(time.Now())
	rc.SetDeadline(time.Now())
	if err != nil {
		return err
	}
	return <-ch
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

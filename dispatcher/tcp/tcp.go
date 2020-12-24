package tcp

import (
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"io"
	"log"
	"net"
	"time"
)

func init() {
	dispatcher.Register("tcp", New)
}

type Dispatcher struct {
	group *config.Group
	l     net.Listener
}

func New(g *config.Group) (d dispatcher.Dispatcher) {
	return &Dispatcher{group: g}
}

func (d *Dispatcher) Listen() (err error) {
	d.l, err = net.Listen("tcp", fmt.Sprintf(":%d", d.group.Port))
	if err != nil {
		return
	}
	defer d.l.Close()
	log.Printf("[tcp] listen on :%v\n", d.group.Port)
	for {
		conn, err := d.l.Accept()
		if err != nil {
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

func (d *Dispatcher) Close() (err error) {
	return d.l.Close()
}

func (d *Dispatcher) handleConn(conn net.Conn) error {
	/*
	   https://github.com/shadowsocks/shadowsocks-org/blob/master/whitepaper/whitepaper.md
	*/
	var (
		server      *config.Server
		userContext *config.UserContext
	)
	defer conn.Close()
	//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
	var buf [32 + 2 + 16]byte
	n, err := io.ReadFull(conn, buf[:])
	if err != nil {
		return fmt.Errorf("[tcp] handleConn readfull error: %v", err)
	}

	// get user's context (preference)
	userContext = d.group.UserContextPool.Get(conn.RemoteAddr(), d.group.Servers)

	// auth every server
	server, _ = d.Auth(buf[:], userContext)
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
	_, err = rc.Write(buf[:n])
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
	ch := make(chan error)
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

func (d *Dispatcher) Auth(data []byte, userContext *config.UserContext) (hit *config.Server, content []byte) {
	if len(data) < 50 {
		return nil, nil //fmt.Errorf("length of data should be no less than 50")
	}
	return userContext.Auth(func(server *config.Server) ([]byte, bool) {
		return probe(data, server)
	})
}

func probe(data []byte, server *config.Server) ([]byte, bool) {
	//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
	conf := cipher.CiphersConf[server.Method]

	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen : conf.SaltLen+2+conf.TagLen]

	content, ok := conf.Verify(server.MasterKey, salt, cipherText)
	return content, ok
}

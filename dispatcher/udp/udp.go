package udp

import (
	"fmt"
	"github.com/Qv2ray/shadomplexer-go/cipher"
	"github.com/Qv2ray/shadomplexer-go/config"
	"github.com/Qv2ray/shadomplexer-go/dispatcher"
	"log"
	"net"
	"time"
)

func init() {
	dispatcher.Register("udp", New)
}

const UDPBufSize = 64 * 1024

type Dispatcher struct {
	group *config.Group
	c     *net.UDPConn
	nm    *UDPConnMapping
}

func New(g *config.Group) (d dispatcher.Dispatcher) {
	return &Dispatcher{group: g, nm: NewUDPConnMapping()}
}

func (d *Dispatcher) Listen() (err error) {
	d.c, err = net.ListenUDP("udp", &net.UDPAddr{Port: d.group.Port})
	if err != nil {
		return
	}
	defer d.c.Close()
	log.Printf("[udp] listen on :%v\n", d.group.Port)
	for {
		buf := make([]byte, UDPBufSize)
		n, laddr, err := d.c.ReadFrom(buf)
		if err != nil {
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		go func() {
			err := d.handleConn(laddr, buf, n)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

func (d *Dispatcher) handleConn(laddr net.Addr, buf []byte, n int) (err error) {
	// get user's context (preference)
	userContext := d.group.UserContextPool.Get(laddr, d.group.Servers)

	// auth every server
	server, err := d.Auth(buf[:n], userContext)
	if err != nil {
		return fmt.Errorf("[udp] handleConn auth error: %v", err)
	}

	// get conn or dial
	rc, err := d.getUCPConn(laddr.String(), server.Target)
	if err != nil {
		return fmt.Errorf("[udp] handleConn dial target error: %v", err)
	}

	// relay
	log.Printf("[udp] %s <-> %s <-> %s", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
	go func() {
		_ = relay(d.c, laddr, rc)
		rc.Close()
	}()
	_, err = rc.Write(buf[:n])
	if err != nil {
		return fmt.Errorf("[udp] handleConn write error: %v", err)
	}
	return nil
}

func (d *Dispatcher) getUCPConn(userIdent string, target string) (rc *net.UDPConn, err error) {
	d.nm.Lock()
	var conn *UDPConn
	var ok bool
	if conn, ok = d.nm.Get(userIdent); !ok {
		d.nm.Insert(userIdent, nil)
		d.nm.Unlock()
		rconn, err := net.Dial("udp", target)
		if err != nil {
			d.nm.Lock()
			d.nm.Remove(userIdent) // close channel to inform that establishment ends
			d.nm.Unlock()
			return nil, fmt.Errorf("getUCPConn dial error: %v", err)
		}
		rc = rconn.(*net.UDPConn)
		d.nm.Lock()
		d.nm.Remove(userIdent) // close channel to inform that establishment ends
		d.nm.Insert(userIdent, rc)
		d.nm.Unlock()
	} else {
		d.nm.Unlock()
		<-conn.Establishing
		if conn.UDPConn == nil {
			// establishment failed
			return d.getUCPConn(userIdent, target)
		} else {
			// establishment succeeded
			rc = conn.UDPConn
		}
	}
	return rc, nil
}

func relay(dst *net.UDPConn, laddr net.Addr, src *net.UDPConn) (err error) {
	var n int
	buf := make([]byte, UDPBufSize)
	_ = src.SetDeadline(time.Now().Add(timeout))
	for {
		n, _, err = src.ReadFrom(buf)
		if err != nil {
			return
		}
		_, err = dst.WriteTo(buf[:n], laddr)
		if err != nil {
			return
		}
	}
}

func (d *Dispatcher) Auth(data []byte, userContext *config.UserContext) (hit *config.Server, err error) {
	if len(data) <= 32 {
		return nil, nil //fmt.Errorf("length of data should be greater than 32")
	}
	return userContext.Auth(func(server *config.Server) bool {
		return probe(data, server)
	})
}

func (d *Dispatcher) Close() (err error) {
	return d.c.Close()
}

func probe(data []byte, server *config.Server) bool {
	//[salt][encrypted payload][tag]
	conf := cipher.CiphersConf[server.Method]
	if len(data) < conf.SaltLen+conf.TagLen {
		return false
	}
	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen:]
	return conf.Verify(server.MasterKey, salt, cipherText)
}

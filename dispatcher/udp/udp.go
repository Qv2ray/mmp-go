package udp

import (
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/common/leakybuf"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"time"
)

func init() {
	dispatcher.Register("udp", New)
}

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
		buf := leakybuf.Get(leakybuf.UDPBufSize)
		n, laddr, err := d.c.ReadFrom(buf)
		if err != nil {
			log.Printf("[error] ReadFrom: %v", err)
			leakybuf.Put(buf)
			continue
		}
		go func() {
			err := d.handleConn(laddr, buf, n)
			if err != nil {
				log.Println(err)
			}
			leakybuf.Put(buf)
		}()
	}
}

// select an appropriate timeout
func selectTimeout(packet []byte) time.Duration {
	var dmessage dnsmessage.Message
	err := dmessage.Unpack(packet)
	if err != nil {
		return defaultTimeout
	}
	return dnsQueryTimeout
}

func (d *Dispatcher) handleConn(laddr net.Addr, data []byte, n int) (err error) {
	// get user's context (preference)
	userContext := d.group.UserContextPool.Get(laddr, d.group.Servers)

	buf := leakybuf.Get(n)
	defer leakybuf.Put(buf)
	// auth every server
	server, content := d.Auth(buf, data[:n], userContext)
	if server == nil {
		return nil
	}
	timeout := selectTimeout(content)
	// get conn or dial and relay
	rc, err := d.getUCPConn(laddr, server.Target, timeout)
	if err != nil {
		return fmt.Errorf("[udp] handleConn dial target error: %v", err)
	}

	// send packet
	log.Printf("[udp] %s <-> %s <-> %s", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
	_, err = rc.Write(data[:n])
	if err != nil {
		return fmt.Errorf("[udp] handleConn write error: %v", err)
	}
	return nil
}

// connTimeout is the timeout of connection to build if not exists
func (d *Dispatcher) getUCPConn(laddr net.Addr, target string, connTimeout time.Duration) (rc *net.UDPConn, err error) {
	socketIdent := laddr.String()
	d.nm.Lock()
	var conn *UDPConn
	var ok bool
	if conn, ok = d.nm.Get(socketIdent); !ok {
		d.nm.Insert(socketIdent, nil, 3600*time.Second)
		d.nm.Unlock()
		rconn, err := net.Dial("udp", target)
		if err != nil {
			d.nm.Lock()
			d.nm.Remove(socketIdent) // close channel to inform that establishment ends
			d.nm.Unlock()
			return nil, fmt.Errorf("getUCPConn dial error: %v", err)
		}
		rc = rconn.(*net.UDPConn)
		d.nm.Lock()
		d.nm.Remove(socketIdent) // close channel to inform that establishment ends
		d.nm.Insert(socketIdent, rc, connTimeout)
		d.nm.Unlock()
		go func() {
			_ = relay(d.c, laddr, rc, connTimeout)
			d.nm.Lock()
			d.nm.Remove(socketIdent)
			d.nm.Unlock()
		}()
	} else {
		d.nm.Unlock()
		<-conn.Establishing
		if conn.UDPConn == nil {
			// establishment ended and retrieve the result
			return d.getUCPConn(laddr, target, connTimeout)
		} else {
			// establishment succeeded before
			rc = conn.UDPConn
		}
	}
	return rc, nil
}

func relay(dst *net.UDPConn, laddr net.Addr, src *net.UDPConn, timeout time.Duration) (err error) {
	var n int
	buf := leakybuf.Get(leakybuf.UDPBufSize)
	defer leakybuf.Put(buf)
	for {
		_ = src.SetReadDeadline(time.Now().Add(timeout))
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

func (d *Dispatcher) Auth(buf []byte, data []byte, userContext *config.UserContext) (hit *config.Server, content []byte) {
	if len(data) <= 32 {
		return nil, nil //fmt.Errorf("length of data should be greater than 32")
	}
	return userContext.Auth(func(server *config.Server) ([]byte, bool) {
		return probe(buf, data, server)
	})
}

func (d *Dispatcher) Close() (err error) {
	return d.c.Close()
}

func probe(buf []byte, data []byte, server *config.Server) ([]byte, bool) {
	//[salt][encrypted payload][tag]
	conf := cipher.CiphersConf[server.Method]
	if len(data) < conf.SaltLen+conf.TagLen {
		return nil, false
	}
	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen:]
	content, ok := conf.Verify(buf, server.MasterKey, salt, cipherText)
	return content, ok
}

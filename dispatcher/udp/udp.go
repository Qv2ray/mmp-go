package udp

import (
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/common/pool"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"time"
)

const (
	BufSize           = 64 * 1024
	BasicLen          = 32
	DefaultNatTimeout = 3 * time.Minute
	DnsQueryTimeout   = 17 * time.Second // RFC 5452
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
	var buf [BufSize]byte
	for {
		n, laddr, err := d.c.ReadFrom(buf[:])
		if err != nil {
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		data := pool.Get(n)
		copy(data, buf[:n])
		go func() {
			err := d.handleConn(laddr, data, n)
			if err != nil {
				log.Println(err)
			}
			pool.Put(data)
		}()
	}
}

func addrLen(packet []byte) int {
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

// select an appropriate timeout
func selectTimeout(packet []byte) time.Duration {
	al := addrLen(packet)
	if len(packet) < al {
		// err: packet with inadequate length
		return DefaultNatTimeout
	}
	packet = packet[al:]
	var dmessage dnsmessage.Message
	if err := dmessage.Unpack(packet); err != nil {
		return DefaultNatTimeout
	}
	return DnsQueryTimeout
}

func (d *Dispatcher) handleConn(laddr net.Addr, data []byte, n int) (err error) {
	// get user's context (preference)
	userContext := d.group.UserContextPool.Get(laddr, d.group.Servers)

	buf := pool.Get(n)
	defer pool.Put(buf)
	// auth every server
	server, content := d.Auth(buf, data[:n], userContext)
	if server == nil {
		return nil
	}
	// get conn or dial and relay
	rc, err := d.GetOrBuildUCPConn(laddr, server.Target, func() time.Duration {
		return selectTimeout(content)
	})
	if err != nil {
		return fmt.Errorf("[udp] handleConn dial target error: %v", err)
	}

	// send packet
	_, err = rc.Write(data[:n])
	if err != nil {
		return fmt.Errorf("[udp] handleConn write error: %v", err)
	}
	return nil
}

// connTimeout is the timeout of connection to build if not exists
func (d *Dispatcher) GetOrBuildUCPConn(laddr net.Addr, target string, natTimeoutFunc func() time.Duration) (rc *net.UDPConn, err error) {
	socketIdent := laddr.String()
	d.nm.Lock()
	var conn *UDPConn
	var ok bool
	if conn, ok = d.nm.Get(socketIdent); !ok {
		// not exist such socket mapping, build one
		d.nm.Insert(socketIdent, nil)
		d.nm.Unlock()
		rconn, err := net.Dial("udp", target)
		if err != nil {
			d.nm.Lock()
			d.nm.Remove(socketIdent) // close channel to inform that establishment ends
			d.nm.Unlock()
			return nil, fmt.Errorf("GetOrBuildUCPConn dial error: %v", err)
		}
		rc = rconn.(*net.UDPConn)
		d.nm.Lock()
		d.nm.Remove(socketIdent) // close channel to inform that establishment ends
		d.nm.Insert(socketIdent, rc)
		d.nm.Unlock()
		// relay
		log.Printf("[udp] %s <-> %s <-> %s", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
		go func() {
			// invoke natTimeoutFunc when necessary
			_ = relay(d.c, laddr, rc, natTimeoutFunc())
			d.nm.Lock()
			d.nm.Remove(socketIdent)
			d.nm.Unlock()
		}()
	} else {
		// exist such socket mapping, just verify or wait for its establishment
		d.nm.Unlock()
		<-conn.Establishing
		if conn.UDPConn == nil {
			// establishment ended and retrieve the result
			return d.GetOrBuildUCPConn(laddr, target, natTimeoutFunc)
		} else {
			// establishment succeeded
			rc = conn.UDPConn
		}
	}
	return rc, nil
}

func relay(dst *net.UDPConn, laddr net.Addr, src *net.UDPConn, timeout time.Duration) (err error) {
	var n int
	buf := pool.Get(BufSize)
	defer pool.Put(buf)
	for {
		_ = src.SetReadDeadline(time.Now().Add(timeout))
		n, _, err = src.ReadFrom(buf)
		if err != nil {
			return
		}
		_ = dst.SetWriteDeadline(time.Now().Add(DefaultNatTimeout)) // should keep consistent
		_, err = dst.WriteTo(buf[:n], laddr)
		if err != nil {
			return
		}
	}
}

func (d *Dispatcher) Auth(buf []byte, data []byte, userContext *config.UserContext) (hit *config.Server, content []byte) {
	if len(data) < BasicLen {
		return nil, nil
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

	return conf.Verify(buf, server.MasterKey, salt, cipherText)
}

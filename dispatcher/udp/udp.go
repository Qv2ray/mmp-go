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
	var buf [leakybuf.UDPBufSize]byte
	for {
		n, laddr, err := d.c.ReadFrom(buf[:])
		if err != nil {
			log.Printf("[error] ReadFrom: %v", err)
			continue
		}
		//data := leakybuf.Get(n)
		data := make([]byte, n)
		copy(data, buf[:n])
		go func() {
			err := d.handleConn(laddr, data, n)
			if err != nil {
				log.Println(err)
			}
			//leakybuf.Put(data)
		}()
	}
}

func addrLen(packet []byte) int {
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
	packet = packet[addrLen(packet):]
	var dmessage dnsmessage.Message
	err := dmessage.Unpack(packet)
	if err != nil {
		return defaultNatTimeout
	}
	return dnsQueryTimeout
}

func (d *Dispatcher) handleConn(laddr net.Addr, data []byte, n int) (err error) {
	// get user's context (preference)
	userContext := d.group.UserContextPool.Get(laddr, d.group.Servers)

	//buf := leakybuf.Get(n)
	//defer leakybuf.Put(buf)
	buf := make([]byte, n)
	// auth every server
	server, content := d.Auth(buf, data[:n], userContext)
	if server == nil {
		return nil
	}
	timeout := selectTimeout(content)
	// get conn or dial and relay
	rc, err := d.GetOrBuildUCPConn(laddr, server.Target, timeout)
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
func (d *Dispatcher) GetOrBuildUCPConn(laddr net.Addr, target string, natTimeout time.Duration) (rc *UDPConn, err error) {
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
		_rconn := rconn.(*net.UDPConn)
		d.nm.Lock()
		d.nm.Remove(socketIdent) // close channel to inform that establishment ends
		d.nm.Insert(socketIdent, _rconn)
		rc, _ = d.nm.Get(socketIdent)
		d.nm.Unlock()
		// relay
		log.Printf("[udp] %s <-> %s <-> %s", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
		go func() {
			_ = relay(d.c, laddr, _rconn, natTimeout)
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
			return d.GetOrBuildUCPConn(laddr, target, natTimeout)
		} else {
			// establishment succeeded
			rc = conn
		}
	}
	return rc, nil
}

func relay(dst *net.UDPConn, laddr net.Addr, src *net.UDPConn, timeout time.Duration) (err error) {
	var n int
	//buf := leakybuf.Get(leakybuf.UDPBufSize)
	//defer leakybuf.Put(buf)
	buf := make([]byte, leakybuf.UDPBufSize)
	for {
		_ = src.SetReadDeadline(time.Now().Add(timeout))
		n, _, err = src.ReadFrom(buf)
		if err != nil {
			return
		}
		_ = dst.SetWriteDeadline(time.Now().Add(timeout))
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

	return conf.Verify(buf, server.MasterKey, salt, cipherText)
}

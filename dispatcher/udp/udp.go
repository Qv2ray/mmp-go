package udp

import (
	"errors"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"github.com/Qv2ray/mmp-go/dispatcher/infra"
	"github.com/Qv2ray/mmp-go/infra/pool"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"sync"
	"time"
)

const (
	MTU               = 65535
	BasicLen          = 32
	DefaultNatTimeout = 3 * time.Minute
	DnsQueryTimeout   = 17 * time.Second // RFC 5452
)

var AuthFailedErr = fmt.Errorf("auth failed")

func init() {
	dispatcher.Register("udp", New)
}

type UDP struct {
	gMutex sync.RWMutex
	group  *config.Group
	c      *net.UDPConn
	nm     *UDPConnMapping
}

func New(g *config.Group) (d dispatcher.Dispatcher) {
	return &UDP{group: g, nm: NewUDPConnMapping()}
}

func (d *UDP) Listen() (err error) {
	d.c, err = net.ListenUDP("udp", &net.UDPAddr{Port: d.group.Port})
	if err != nil {
		return
	}
	defer d.c.Close()
	log.Printf("[udp] listen on :%v\n", d.group.Port)
	var buf [MTU]byte
	for {
		n, laddr, err := d.c.ReadFrom(buf[:])
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
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

func (d *UDP) UpdateGroup(group *config.Group) {
	d.gMutex.Lock()
	defer d.gMutex.Unlock()
	d.group = group
}

func (d *UDP) handleConn(laddr net.Addr, data []byte, n int) (err error) {
	// get conn or dial and relay
	rc, err := d.GetOrBuildUCPConn(laddr, data[:n])
	if err != nil {
		if err == AuthFailedErr {
			return nil
		}
		return fmt.Errorf("[udp] handleConn dial target error: %v", err)
	}

	// send packet
	if _, err = rc.Write(data[:n]); err != nil {
		return fmt.Errorf("[udp] handleConn write error: %v", err)
	}
	return nil
}

// select an appropriate timeout
func selectTimeout(packet []byte) time.Duration {
	al := infra.AddrLen(packet)
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

// connTimeout is the timeout of connection to build if not exists
func (d *UDP) GetOrBuildUCPConn(laddr net.Addr, data []byte) (rc *net.UDPConn, err error) {
	socketIdent := laddr.String()
	d.nm.Lock()
	var conn *UDPConn
	var ok bool
	if conn, ok = d.nm.Get(socketIdent); !ok {
		// not exist such socket mapping, build one
		d.nm.Insert(socketIdent, nil)
		d.nm.Unlock()

		// get user's context (preference)
		d.gMutex.RLock() // avoid insert old servers to the new userContextPool
		userContext := d.group.UserContextPool.GetOrInsert(laddr, d.group.Servers)
		d.gMutex.RUnlock()

		buf := pool.Get(len(data))
		defer pool.Put(buf)
		// auth every server
		server, content := d.Auth(buf, data, userContext)
		if server == nil {
			return nil, AuthFailedErr
		}

		// dial
		rconn, err := net.Dial("udp", server.Target)
		if err != nil {
			d.nm.Lock()
			d.nm.Remove(socketIdent) // close channel to inform that establishment ends
			d.nm.Unlock()
			return nil, fmt.Errorf("GetOrBuildUCPConn dial error: %v", err)
		}
		rc = rconn.(*net.UDPConn)
		d.nm.Lock()
		d.nm.Remove(socketIdent) // close channel to inform that establishment ends
		conn = d.nm.Insert(socketIdent, rc)
		conn.timeout = selectTimeout(content)
		d.nm.Unlock()
		// relay
		log.Printf("[udp] %s <-> %s <-> %s", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
		go func() {
			_ = relay(d.c, laddr, rc, conn.timeout)
			d.nm.Lock()
			d.nm.Remove(socketIdent)
			d.nm.Unlock()
		}()
	} else {
		// such socket mapping exists; just verify or wait for its establishment
		d.nm.Unlock()
		<-conn.Establishing
		if conn.UDPConn == nil {
			// establishment ended and retrieve the result
			return d.GetOrBuildUCPConn(laddr, data)
		} else {
			// establishment succeeded
			rc = conn.UDPConn
		}
	}
	// countdown
	_ = conn.UDPConn.SetReadDeadline(time.Now().Add(conn.timeout))
	return rc, nil
}

func relay(dst *net.UDPConn, laddr net.Addr, src *net.UDPConn, timeout time.Duration) (err error) {
	var n int
	buf := pool.Get(MTUTrie.GetMTU(src.LocalAddr().(*net.UDPAddr).IP))
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

func (d *UDP) Auth(buf []byte, data []byte, userContext *config.UserContext) (hit *config.Server, content []byte) {
	if len(data) < BasicLen {
		return nil, nil
	}
	return userContext.Auth(func(server *config.Server) ([]byte, bool) {
		return probe(buf, data, server)
	})
}

func (d *UDP) Close() (err error) {
	log.Printf("[udp] closed :%v\n", d.group.Port)
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

	subKey := pool.Get(conf.KeyLen)[:0]
	defer pool.Put(subKey)
	if !conf.UnsafeVerifyATyp(buf, server.MasterKey, salt, cipherText, &subKey) {
		return nil, false
	}
	return conf.Verify(buf, server.MasterKey, salt, cipherText, &subKey)
}

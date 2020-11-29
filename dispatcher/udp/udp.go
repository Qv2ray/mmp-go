package udp

import (
	"crypto/sha1"
	"fmt"
	"github.com/Qv2ray/shadomplexer-go/cipher"
	"github.com/Qv2ray/shadomplexer-go/config"
	"github.com/Qv2ray/shadomplexer-go/dispatcher"
	"golang.org/x/crypto/hkdf"
	"io"
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
	userIdent, _, _ := net.SplitHostPort(laddr.String())
	userContext := d.group.UserContextPool.GetOrInsert(userIdent, func() (val interface{}) {
		return config.NewUserContext(d.group.Servers)
	}).Val.(*config.UserContext)

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
	log.Printf("[udp] %s <-> %s <-> %s ", laddr.String(), d.c.LocalAddr(), rc.RemoteAddr())
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
	defer d.nm.Unlock()
	var ok bool
	// TODO: optimize
	if rc, ok = d.nm.Get(userIdent); !ok {
		rconn, err := net.Dial("udp", target)
		if err != nil {
			return nil, fmt.Errorf("getUCPConn dial error: %v", err)
		}
		rc = rconn.(*net.UDPConn)
		d.nm.Insert(userIdent, rc)
	}
	return nil, nil
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
	ctx := userContext.Infra()
	// probe every server
	for serverNode := ctx.Front(); serverNode != ctx.Tail(); serverNode = serverNode.Next() {
		server := serverNode.Val.(*config.Server)
		if probe(data, server) {
			ctx.Promote(serverNode)
			return server, nil
		}
	}
	return nil, nil
}

func (d *Dispatcher) Close() (err error) {
	return d.c.Close()
}

func probe(data []byte, server *config.Server) bool {
	conf := cipher.CiphersConf[server.Method]
	if len(data) < conf.SaltLen+conf.TagLen {
		return false
	}
	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen:]
	kdf := hkdf.New(sha1.New, server.MasterKey[:conf.KeyLen], salt, []byte("ss-subkey"))
	subKey := make([]byte, conf.KeyLen)
	io.ReadFull(kdf, subKey)
	ciph, _ := conf.NewCipher(subKey)
	buf := make([]byte, UDPBufSize)
	_, err := ciph.Open(buf, dispatcher.ZeroNonce[:conf.NonceLen], cipherText, nil)
	return err == nil
}

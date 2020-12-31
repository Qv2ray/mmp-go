package udp

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/config"
	"golang.org/x/crypto/hkdf"
	"math/rand"
	"net"
	"testing"
)

func BenchmarkDispatcher_Auth(b *testing.B) {
	const nServers = 100
	g := new(config.Group)
	for i := 0; i < nServers; i++ {
		var b [10]byte
		rand.Read(b[:])
		g.Servers = append(g.Servers, config.Server{
			Target:   "127.0.0.1:1080",
			Method:   "chacha20-ietf-poly1305",
			Password: string(b[:]),
		})
	}
	g.BuildMasterKeys()
	g.BuildUserContextPool(10)
	var buf [65535]byte
	var data [65535]byte
	var d = New(g)
	addr, _ := net.ResolveIPAddr("udp", "127.0.0.1:50000")
	for i := 0; i < b.N; i++ {
		d.Auth(buf[:], data[:], g.UserContextPool.GetOrInsert(addr, g.Servers))
	}
}

// TestDispatcher_Auth just test if single server auth works
func TestDispatcher_Auth(t *testing.T) {
	const testSize = 500000
	const method = "chacha20-ietf-poly1305"
	const password = "password"

	g := new(config.Group)
	g.Servers = []config.Server{{
		Target:   "127.0.0.1:1080",
		Method:   method,
		Password: password,
	}}
	g.BuildMasterKeys()
	g.BuildUserContextPool(10)
	var d = New(g)

	type test struct {
		data     []byte
		positive bool
	}

	var tests []test
	var salt [32]byte
	for i := 0; i < testSize; i++ {
		rand.Read(salt[:])
		atyp := rand.Intn(6) //0 2 5 is invalid atyp
		var addr []byte
		var tt test
		switch atyp {
		case cipher.ATypeIPv4:
			addr = make([]byte, 1+4+2)
			copy(addr[1:], net.IPv4(byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255))).To4())
			binary.BigEndian.PutUint16(addr[1+4:], uint16(rand.Intn(65536)))
		case cipher.ATypeIpv6:
			addr = make([]byte, 1+16+2)
			// v4 in v6; fake v6
			copy(addr[1:], net.IPv4(byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255)), byte(rand.Intn(255))))
			binary.BigEndian.PutUint16(addr[1+16:], uint16(rand.Intn(65536)))
		case cipher.ATypeDomain:
			dm := "apple.com"
			addr = make([]byte, 1+1+len(dm)+2)
			addr[1] = byte(len(dm))
			copy(addr[1+1:], dm)
			binary.BigEndian.PutUint16(addr[1+1+len(dm):], uint16(rand.Intn(65536)))
		default:
			addr = make([]byte, 1+6+rand.Intn(16))
		loop:
			for {
				switch addr[0] {
				case cipher.ATypeIPv4, cipher.ATypeDomain, cipher.ATypeIpv6:
					rand.Read(addr)
				default:
					break loop
				}
			}
		}
		var payload = make([]byte, 16+rand.Intn(512))
		rand.Read(payload)

		switch atyp {
		case cipher.ATypeIPv4, cipher.ATypeDomain, cipher.ATypeIpv6:
			tt.positive = true
			addr[0] = byte(atyp)

			conf := cipher.CiphersConf[method]
			kdf := hkdf.New(sha1.New, g.Servers[0].MasterKey, salt[:], cipher.ReusedInfo)
			sk := make([]byte, conf.KeyLen)
			kdf.Read(sk)
			aead, _ := conf.NewCipher(sk)
			dataLen := len(addr) + len(payload) + aead.Overhead()
			data := make([]byte, dataLen)
			aead.Seal(data[:0], cipher.ZeroNonce[:aead.NonceSize()], bytes.Join([][]byte{addr, payload}, nil), nil)
			tt.data = bytes.Join([][]byte{salt[:], data[:dataLen]}, nil)
		default:
			tt.data = bytes.Join([][]byte{salt[:], addr, payload}, nil)
		}
		tests = append(tests, tt)
	}

	var buf [65535]byte
	addr, _ := net.ResolveIPAddr("udp", "127.0.0.1:50000")
	for _, test := range tests {
		hit, _ := d.Auth(buf[:], test.data, g.UserContextPool.GetOrInsert(addr, g.Servers))
		valid := hit != nil
		if valid != test.positive {
			t.Fail()
		}
	}
}

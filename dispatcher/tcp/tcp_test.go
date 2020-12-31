package tcp

import (
	"github.com/Qv2ray/mmp-go/config"
	"math/rand"
	"net"
	"testing"
)

func BenchmarkAuth(b *testing.B) {
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
	var buf [50]byte
	var data [50]byte
	var d = New(g)
	addr, _ := net.ResolveIPAddr("tcp", "127.0.0.1:50000")
	for i := 0; i < b.N; i++ {
		d.Auth(buf[:], data[:], g.UserContextPool.GetOrInsert(addr, g.Servers))
	}
}

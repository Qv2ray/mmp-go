package config

import (
	"github.com/Qv2ray/mmp-go/infra/lru"
	"github.com/Qv2ray/mmp-go/infra/lrulist"
	"math/rand"
	"net"
	"time"
)

// encapsulating semantic types
type UserContext lrulist.LruList
type UserContextPool lru.LRU

func NewUserContext(servers []Server) *UserContext {
	list := make([]interface{}, len(servers))
	for i := range servers {
		list[i] = &servers[i]
	}
	basicInterval := 10 * time.Second
	offsetRange := 6.0
	offset := time.Duration((rand.Float64()-0.5)*offsetRange*1000) * time.Millisecond
	ctx := lrulist.NewWithList(basicInterval+offset, lrulist.InsertFront, list)
	return (*UserContext)(ctx)
}

func (ctx *UserContext) Infra() *lrulist.LruList {
	return (*lrulist.LruList)(ctx)
}

func (ctx *UserContext) Close() error {
	return ctx.Infra().Close()
}

func (ctx *UserContext) Auth(probe func(*Server) ([]byte, bool)) (hit *Server, content []byte) {
	lruList := ctx.Infra()
	listCopy := lruList.GetListCopy()
	defer lruList.GiveBackListCopy(listCopy)
	// probe every server
	for i := range listCopy {
		server := listCopy[i].Val.(*Server)
		if content, ok := probe(server); ok {
			lruList.Promote(listCopy[i])
			return server, content
		}
	}
	return nil, nil
}

func (pool *UserContextPool) Infra() *lru.LRU {
	return (*lru.LRU)(pool)
}

func (pool *UserContextPool) GetOrInsert(addr net.Addr, servers []Server) *UserContext {
	userIdent, _, _ := net.SplitHostPort(addr.String())
	value, removed := pool.Infra().GetOrInsert(userIdent, func() (val interface{}) {
		return NewUserContext(servers)
	})
	for _, ev := range removed {
		ev.Value.(*UserContext).Close()
	}
	return value.(*UserContext)
}

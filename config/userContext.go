package config

import (
	"github.com/Qv2ray/mmp-go/common/lru"
	"github.com/Qv2ray/mmp-go/common/lrulist"
	"net"
	"time"
)

// encapsulating semantic types
type UserContext lrulist.LruList
type UserContextPool lru.LRU

func NewUserContext(servers []Server) *UserContext {
	list := make([]interface{}, len(servers))
	for i := range servers {
		list[i] = servers[i]
	}
	ctx := lrulist.NewWithList(10*time.Second, lrulist.InsertFront, list)
	return (*UserContext)(ctx)
}

func (ctx *UserContext) Infra() *lrulist.LruList {
	return (*lrulist.LruList)(ctx)
}

func (ctx *UserContext) Close() error {
	return ctx.Infra().Close()
}

func (ctx *UserContext) Auth(probe func(*Server) bool) (hit *Server, err error) {
	lruList := ctx.Infra()
	listCopy := lruList.GetListCopy()
	// probe every server
	for _, serverNode := range listCopy {
		server := serverNode.Val.(Server)
		if probe(&server) {
			lruList.Promote(serverNode)
			return &server, nil
		}
	}
	return nil, nil
}

func (pool *UserContextPool) Infra() *lru.LRU {
	return (*lru.LRU)(pool)
}

func (pool *UserContextPool) Get(addr net.Addr, servers []Server) *UserContext {
	userIdent, _, _ := net.SplitHostPort(addr.String())
	node, removed := pool.Infra().GetOrInsert(userIdent, func() (val interface{}) {
		return NewUserContext(servers)
	})
	if removed != nil {
		removed.Val.(*UserContext).Close()
	}
	return node.Val.(*UserContext)
}

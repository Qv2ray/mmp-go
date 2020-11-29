package config

import (
	"github.com/Qv2ray/shadomplexer-go/common/linklist"
	"github.com/Qv2ray/shadomplexer-go/common/lru"
	"net"
)

// encapsulating a semantic type
type UserContext linklist.Linklist
type UserContextPool lru.LRU

func NewUserContext(servers []Server) *UserContext {
	ctx := linklist.NewLinklist()
	for i := range servers {
		ctx.PushBack(&servers[i])
	}
	return (*UserContext)(ctx)
}

func (ctx *UserContext) Infra() *linklist.Linklist {
	return (*linklist.Linklist)(ctx)
}

func (ctx *UserContext) Auth(probe func(*Server) bool) (hit *config.Server, err error) {
	list := ctx.Infra()
	// probe every server
	for serverNode := list.Front(); serverNode != list.Tail(); serverNode = serverNode.Next() {
		server := serverNode.Val.(*Server)
		if probe(server) {
			list.Promote(serverNode)
			return server, nil
		}
	}
	return nil, nil
}

func (pool *UserContextPool) Infra() *lru.LRU {
	return (*lru.LRU)(pool)
}

func (pool *UserContextPool) Get(addr net.Addr, servers []Server) *UserContext {
	userIdent, _, _ := net.SplitHostPort(addr.String())
	return pool.Infra().GetOrInsert(userIdent, func() (val interface{}) {
		return NewUserContext(servers)
	}).Val.(*UserContext)
}

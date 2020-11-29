package config

import "github.com/Qv2ray/shadomplexer-go/common/linklist"

// encapsulating a semantic type
type UserContext linklist.Linklist

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

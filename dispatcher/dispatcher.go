package dispatcher

import (
	"github.com/Qv2ray/shadomplexer-go/config"
	"sync"
)

type Dispatcher interface {
	Listen() (err error)
	Auth(data []byte, userContext *config.UserContext) (hit *config.Server, err error)
	Close() (err error)
}

var ZeroNonce [128]byte

type DispatcherCreator func(group *config.Group) Dispatcher

var mapDispatherCreator sync.Map

func Register(name string, creator DispatcherCreator) {
	mapDispatherCreator.Store(name, creator)
}

func New(name string, group *config.Group) (Dispatcher, bool) {
	c, ok := mapDispatherCreator.Load(name)
	if !ok {
		return nil, false
	}
	creator := c.(DispatcherCreator)
	return creator(group), ok
}

package dispatcher

import (
	"github.com/Qv2ray/mmp-go/config"
	"sync"
)

type Dispatcher interface {
	Listen() (err error)
	// buf is a buffer to store decrypted text
	Auth(buf []byte, data []byte, userContext *config.UserContext) (hit *config.Server, content []byte)
	UpdateGroup(group *config.Group)
	Close() (err error)
}

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

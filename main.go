package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	_ "github.com/Qv2ray/mmp-go/dispatcher/tcp"
	_ "github.com/Qv2ray/mmp-go/dispatcher/udp"
	"log"
	"sync"
)

type MapPortDispatcher map[int]*[len(protocols)]dispatcher.Dispatcher

type SyncMapPortDispatcher struct {
	sync.Mutex
	Map MapPortDispatcher
}

func NewSyncMapPortDispatcher() *SyncMapPortDispatcher {
	return &SyncMapPortDispatcher{Map: make(MapPortDispatcher)}
}

var (
	protocols       = [...]string{"tcp", "udp"}
	groupWG         sync.WaitGroup
	mPortDispatcher = NewSyncMapPortDispatcher()
)

func listenGroup(group *config.Group) {
	err := listenProtocols(group, protocols[:])
	if err != nil {
		mPortDispatcher.Lock()
		// error but listening
		if _, ok := mPortDispatcher.Map[group.Port]; ok {
			log.Fatalln(err)
		}
		mPortDispatcher.Unlock()
	}
}

func listenProtocols(group *config.Group, protocols []string) error {
	mPortDispatcher.Lock()
	if _, ok := mPortDispatcher.Map[group.Port]; !ok {
		mPortDispatcher.Map[group.Port] = new([2]dispatcher.Dispatcher)
	}
	t := mPortDispatcher.Map[group.Port]
	mPortDispatcher.Unlock()

	ch := make(chan error, len(protocols))
	for i, protocol := range protocols {
		d, _ := dispatcher.New(protocol, group)
		(*t)[i] = d
		go func() {
			var err error
			err = d.Listen()
			ch <- err
		}()
	}
	return <-ch
}

func main() {
	// handle reload
	go signalHandler()

	mPortDispatcher.Lock()
	conf := config.GetConfig()
	for i := range conf.Groups {
		groupWG.Add(1)
		go func(group *config.Group) {
			listenGroup(group)
			groupWG.Done()
		}(&conf.Groups[i])
	}
	mPortDispatcher.Unlock()
	groupWG.Wait()
}

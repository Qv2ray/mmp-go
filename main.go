package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	_ "github.com/Qv2ray/mmp-go/dispatcher/tcp"
	_ "github.com/Qv2ray/mmp-go/dispatcher/udp"
	"log"
	"sync"
)

var protocols = [...]string{"tcp", "udp"}
var wg sync.WaitGroup

func main() {
	if config.DaemonMode {
		// handle reload
		go signalHandler()
	}

	mMutex.Lock()
	conf := config.GetConfig()
	for i := range conf.Groups {
		wg.Add(1)
		go listen(&conf.Groups[i])
	}
	mMutex.Unlock()
	wg.Wait()
	log.Println("quit")
}

func listen(group *config.Group) {
	mMutex.Lock()
	if _, ok := mPortDispatcher[group.Port]; !ok {
		mPortDispatcher[group.Port] = new([2]dispatcher.Dispatcher)
	}
	mMutex.Unlock()
	err := _listen(group, protocols[:])
	if err != nil {
		// listening but error
		mMutex.Lock()
		if _, ok := mPortDispatcher[group.Port]; ok {
			log.Fatalln(err)
		}
		mMutex.Unlock()
	}
	wg.Done()
}

func _listen(group *config.Group, protocols []string) error {
	ch := make(chan error, len(protocols))
	for i, protocol := range protocols {
		d, _ := dispatcher.New(protocol, group)
		t := mPortDispatcher[group.Port]
		(*t)[i] = d
		go func() {
			var err error
			err = d.Listen()
			ch <- err
		}()
	}
	return <-ch
}

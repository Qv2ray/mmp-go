package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	_ "github.com/Qv2ray/mmp-go/dispatcher/tcp"
	_ "github.com/Qv2ray/mmp-go/dispatcher/udp"
	"log"
	"net/http"
	_ "net/http/pprof"
	"sync"
)

var protocols = [...]string{"tcp", "udp"}
var wg sync.WaitGroup

func main() {
	// handle reload
	go signalHandler()

	mMutex.Lock()
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	conf := config.GetConfig()
	for i := range conf.Groups {
		wg.Add(1)
		go listen(&conf.Groups[i])
	}
	mMutex.Unlock()
	wg.Wait()
}

func listen(group *config.Group) {
	err := listenWithProtocols(group, protocols[:])
	if err != nil {
		mMutex.Lock()
		// error but listening
		if _, ok := mPortDispatcher[group.Port]; ok {
			log.Fatalln(err)
		}
		mMutex.Unlock()
	}
	wg.Done()
}

func listenWithProtocols(group *config.Group, protocols []string) error {
	mMutex.Lock()
	if _, ok := mPortDispatcher[group.Port]; !ok {
		mPortDispatcher[group.Port] = new([2]dispatcher.Dispatcher)
	}
	t := mPortDispatcher[group.Port]
	mMutex.Unlock()

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

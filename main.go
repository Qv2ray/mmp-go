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

func main() {
	go func() {
		http.ListenAndServe("0.0.0.0:6060", nil)
	}()
	conf := config.GetConfig()
	var wg sync.WaitGroup
	for i := range conf.Groups {
		wg.Add(1)
		go func(group *config.Group) {
			err := listen(group, protocols[:])
			if err != nil {
				log.Fatalln(err)
			}
			wg.Done()
		}(&conf.Groups[i])
	}
	wg.Wait()
}

func listen(group *config.Group, protocols []string) error {
	ch := make(chan error, len(protocols))
	for _, protocol := range protocols {
		d, _ := dispatcher.New(protocol, group)
		go func() {
			var err error
			err = d.Listen()
			ch <- err
		}()
	}
	return <-ch
}

package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"log"
	"sync"
)

var mMutex sync.Mutex
var mPortDispatcher = make(map[int]*[len(protocols)]dispatcher.Dispatcher)

func ReloadConfig() {
	log.Println("configure reloading")
	mMutex.Lock()
	defer mMutex.Unlock()

	// rebuild config
	confPath := config.GetConfig().ConfPath
	newConf, err := config.BuildConfig(confPath)
	if err != nil {
		log.Printf("failed to reload: %v", err)
		return
	}
	config.SetConfig(newConf)
	c := newConf

	// update dispatchers
	newConfPortSet := make(map[int]struct{})
	for i := range c.Groups {
		newConfPortSet[c.Groups[i].Port] = struct{}{}

		if t, ok := mPortDispatcher[c.Groups[i].Port]; ok {
			// update the existing dispatcher
			for j := range protocols {
				t[j].UpdateGroup(&c.Groups[i])
			}
		} else {
			// add a new port dispatcher
			wg.Add(1)
			go listen(&c.Groups[i])
		}
	}
	// close all removed port dispatcher
	for port := range mPortDispatcher {
		if _, ok := newConfPortSet[port]; !ok {
			t := mPortDispatcher[port]
			delete(mPortDispatcher, port)
			for j := range protocols {
				_ = (*t)[j].Close()
			}
		}
	}
	log.Println("configuration reloaded")
}

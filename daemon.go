package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var mMutex sync.Mutex
var mPortDispatcher = make(map[int]*[len(protocols)]dispatcher.Dispatcher)

func signalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGTERM)
	for sig := range ch {
		switch sig {
		case syscall.SIGTERM:
			log.Println("stopped")
			os.Exit(0)
		case syscall.SIGHUP:
			ReloadConfig()
		}
	}
}
func ReloadConfig() {
	log.Println("configure reloading")
	mMutex.Lock()
	defer mMutex.Unlock()

	// rebuild config
	confPath := config.GetConfig().ConfPath
	config.SetConfig(config.BuildConfig(confPath))
	c := config.GetConfig()

	// update dispatchers
	m := make(map[int]struct{})
	for i := range c.Groups {
		m[c.Groups[i].Port] = struct{}{}

		if t, ok := mPortDispatcher[c.Groups[i].Port]; ok {
			// update existing dispatcher
			for j := range protocols {
				t[j].UpdateGroup(&c.Groups[i])
			}
		} else {
			// add new port dispatcher
			wg.Add(1)
			go listen(&c.Groups[i])
		}
	}
	// close all removed port dispatcher
	for port := range mPortDispatcher {
		if _, ok := m[port]; !ok {
			t := mPortDispatcher[port]
			delete(mPortDispatcher, port)
			for j := range protocols {
				_ = (*t)[j].Close()
			}
		}
	}
	log.Println("configure reloaded")
}

package main

import (
	"github.com/Qv2ray/mmp-go/config"
	"github.com/Qv2ray/mmp-go/dispatcher"
	"log"
	"net"
	"sync"
)

var mMutex sync.Mutex
var mPortDispatcher = make(map[int]*[len(protocols)]dispatcher.Dispatcher)

func ReloadConfig() {
	log.Println("Reloading configuration")
	mMutex.Lock()
	defer mMutex.Unlock()

	// rebuild config
	confPath := config.GetConfig().ConfPath
	oldConf := config.GetConfig()
	newConf, err := config.BuildConfig(confPath)
	if err != nil {
		log.Printf("failed to reload configuration: %v", err)
		return
	}
	// check if there is any net error when pulling the upstream configurations
	for i := range newConf.Groups {
		newGroup := newConf.Groups[i]
		for j := range newGroup.Upstreams {
			newUpstream := newGroup.Upstreams[j]
			if _, ok := newUpstream.PullingError.(net.Error); !ok {
				continue
			}
			// net error, remain those servers

			// find the group in the oldConf
			var oldGroup *config.Group
			for k := range oldConf.Groups {
				// they should have the same port
				if oldConf.Groups[k].Port != newGroup.Port {
					continue
				}
				oldGroup = &oldConf.Groups[k]
			}
			if oldGroup == nil {
				// cannot find the corresponding old group
				continue
			}
			newUpstreamHash := newUpstream.Hash()
			// check if hashes can match
			for k := range oldGroup.Servers {
				oldServer := oldGroup.Servers[k]
				if oldServer.UpstreamHash == newUpstreamHash {
					newGroup.Servers = append(newGroup.Servers, oldServer)
				}
			}
		}
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
	log.Println("Reloaded configuration")
}

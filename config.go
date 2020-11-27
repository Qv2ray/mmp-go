package main

import (
	"encoding/json"
	"fmt"
	"github.com/Qv2ray/shadomplexer-go/common/lru"
	"io/ioutil"
	"log"
	"sync"
)

type Config struct {
	Groups  []Group `json:"groups"`
	LRUSize int     `json:"lruSize"`
}
type Server struct {
	Target    string `json:"target"`
	Method    string `json:"method"`
	Password  string `json:"password"`
	MasterKey []byte `json:"-"`
}
type Group struct {
	Port        int      `json:"port"`
	Servers     []Server `json:"servers"`
	LRUSize     int      `json:"lruSize"`
	UserContext *lru.LRU `json:"-"`
}

var config *Config
var once sync.Once

func build(config *Config) {
	globalLRUSize := config.LRUSize
	if globalLRUSize == 0 {
		globalLRUSize = DefaultLRUSize
	}
	for i := range config.Groups {
		g := &config.Groups[i]
		lruSize := g.LRUSize
		if lruSize == 0 {
			lruSize = globalLRUSize
		}
		g.UserContext = lru.New(lruSize)
		for j := range config.Groups[i].Servers {
			s := &config.Groups[i].Servers[j]
			s.MasterKey = EVPBytesToKey(s.Password, CiphersConf[s.Method].KeyLen)
		}
	}
}

func checkMethodSupported(config *Config) error {
	for _, g := range config.Groups {
		for _, s := range g.Servers {
			if _, ok := CiphersConf[s.Method]; !ok {
				return fmt.Errorf("unsupported method: %v", s.Method)
			}
		}
	}
	return nil
}

func GetConfig() *Config {
	once.Do(func() {
		config = new(Config)
		b, err := ioutil.ReadFile("example.json")
		if err != nil {
			log.Fatalln(err)
		}
		err = json.Unmarshal(b, config)
		if err != nil {
			log.Fatalln(err)
		}
		if err = checkMethodSupported(config); err != nil {
			log.Fatalln(err)
		}
		build(config)
	})
	return config
}

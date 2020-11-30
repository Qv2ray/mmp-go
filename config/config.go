package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Qv2ray/shadomplexer-go/cipher"
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
	Port            int              `json:"port"`
	Servers         []Server         `json:"servers"`
	LRUSize         int              `json:"lruSize"`
	UserContextPool *UserContextPool `json:"-"`
}

var config *Config
var once sync.Once

const (
	DefaultLRUSize = 30
)

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
		g.UserContextPool = (*UserContextPool)(lru.New(lruSize))
		for j := range config.Groups[i].Servers {
			s := &config.Groups[i].Servers[j]
			s.MasterKey = cipher.EVPBytesToKey(s.Password, cipher.CiphersConf[s.Method].KeyLen)
		}
	}
}

func checkMethodSupported(config *Config) error {
	for _, g := range config.Groups {
		for _, s := range g.Servers {
			if _, ok := cipher.CiphersConf[s.Method]; !ok {
				return fmt.Errorf("unsupported method: %v", s.Method)
			}
		}
	}
	return nil
}

func GetConfig() *Config {
	once.Do(func() {
		filename := flag.String("conf", "example.json", "config file path")
		flag.Parse()

		config = new(Config)
		b, err := ioutil.ReadFile(*filename)
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

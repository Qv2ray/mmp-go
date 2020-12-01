package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/common/lru"
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
		buildUserContextPool(g, globalLRUSize)
		buildMasterKeys(g.Servers)
	}
}
func buildMasterKeys(servers []Server) {
	for j := range servers {
		s := &servers[j]
		s.MasterKey = cipher.EVPBytesToKey(s.Password, cipher.CiphersConf[s.Method].KeyLen)
	}
}
func buildUserContextPool(g *Group, globalLRUSize int) {
	lruSize := g.LRUSize
	if lruSize == 0 {
		lruSize = globalLRUSize
	}
	g.UserContextPool = (*UserContextPool)(lru.New(lruSize))
}

func check(config *Config) (err error) {
	if err = checkMethodSupported(config); err != nil {
		return
	}
	if err = checkDiverseCombinations(config.Groups); err != nil {
		return
	}
	return
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
func checkDiverseCombinations(groups []Group) error {
	type methodPasswd struct {
		method string
		passwd string
	}
	for _, g := range groups {
		m := make(map[methodPasswd]struct{})
		for _, s := range g.Servers {
			mp := methodPasswd{
				method: s.Method,
				passwd: s.Password,
			}
			if _, exists := m[mp]; exists {
				return fmt.Errorf("make sure combinantions of method and password in the same group are diverse. counterexample: (%v,%v)", mp.method, mp.passwd)
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
		if err = check(config); err != nil {
			log.Fatalln(err)
		}
		build(config)
	})
	return config
}

package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/common/lru"
	"io/ioutil"
	"log"
	"os"
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
	Port            int                 `json:"port"`
	Servers         []Server            `json:"servers"`
	Upstreams       []map[string]string `json:"upstreams"`
	LRUSize         int                 `json:"lruSize"`
	UserContextPool *UserContextPool    `json:"-"`
}

var config *Config
var once sync.Once
var Version = "debug"

const (
	DefaultLRUSize = 50
)

func (g *Group) BuildMasterKeys() {
	servers := g.Servers
	for j := range servers {
		s := &servers[j]
		s.MasterKey = cipher.EVPBytesToKey(s.Password, cipher.CiphersConf[s.Method].KeyLen)
	}
}
func (g *Group) BuildUserContextPool(globalLRUSize int) {
	lruSize := g.LRUSize
	if lruSize == 0 {
		lruSize = globalLRUSize
	}
	g.UserContextPool = (*UserContextPool)(lru.New(lruSize))
}

func (config *Config) CheckMethodSupported() error {
	for _, g := range config.Groups {
		for _, s := range g.Servers {
			if _, ok := cipher.CiphersConf[s.Method]; !ok {
				return fmt.Errorf("unsupported method: %v", s.Method)
			}
		}
	}
	return nil
}
func (config *Config) CheckDiverseCombinations() error {
	groups := config.Groups
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
func parseUpstreams(config *Config) (err error) {
	for i := range config.Groups {
		g := &config.Groups[i]
		for j, u := range g.Upstreams {
			var upstream Upstream
			switch u["type"] {
			case "outline":
				var outline Outline
				err = Map2upstream(u, &outline)
				if err != nil {
					return
				}
				upstream = outline
			default:
				return fmt.Errorf("unknown upstream type: %v", u["type"])
			}
			servers, err := upstream.GetServers()
			if err != nil {
				log.Printf("[warning] Failed to retrieve configure from groups[%d].upstreams[%d]: %v\n", i, j, err)
				continue
			}
			g.Servers = append(g.Servers, servers...)
		}
	}
	return nil
}
func check(config *Config) (err error) {
	if err = config.CheckMethodSupported(); err != nil {
		return
	}
	if err = config.CheckDiverseCombinations(); err != nil {
		return
	}
	return
}
func build(config *Config) {
	globalLRUSize := config.LRUSize
	if globalLRUSize == 0 {
		globalLRUSize = DefaultLRUSize
	}
	for i := range config.Groups {
		g := &config.Groups[i]
		g.BuildUserContextPool(globalLRUSize)
		g.BuildMasterKeys()
	}
}
func GetConfig() *Config {
	once.Do(func() {
		version := flag.Bool("v", false, "version")
		filename := flag.String("conf", "example.json", "config file path")
		flag.Parse()

		if *version {
			fmt.Println(Version)
			os.Exit(0)
		}

		config = new(Config)
		b, err := ioutil.ReadFile(*filename)
		if err != nil {
			log.Fatalln(err)
		}
		if err = json.Unmarshal(b, config); err != nil {
			log.Fatalln(err)
		}
		log.Println("pulling configures from upstreams...")
		if err = parseUpstreams(config); err != nil {
			log.Fatalln(err)
		}
		if err = check(config); err != nil {
			log.Fatalln(err)
		}
		build(config)
	})
	return config
}

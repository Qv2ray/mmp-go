package config

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/infra/lru"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type Config struct {
	ConfPath string  `json:"-"`
	Groups   []Group `json:"groups"`
}
type Server struct {
	Target       string        `json:"target"`
	Method       string        `json:"method"`
	Password     string        `json:"password"`
	MasterKey    []byte        `json:"-"`
	UpstreamConf *UpstreamConf `json:"-"`
}
type Group struct {
	Port            int              `json:"port"`
	Servers         []Server         `json:"servers"`
	Upstreams       []UpstreamConf   `json:"upstreams"`
	LRUSize         int              `json:"lruSize"`
	UserContextPool *UserContextPool `json:"-"`
}
type UpstreamConf map[string]string

const (
	PullingErrorKey      = "__pulling_error__"
	PullingErrorNetError = "net_error"
)

func (uc UpstreamConf) InitPullingError() {
	if _, ok := uc[PullingErrorKey]; !ok {
		uc[PullingErrorKey] = ""
	}
}

func (uc UpstreamConf) Equal(that UpstreamConf) bool {
	uc.InitPullingError()
	that.InitPullingError()
	if len(uc) != len(that) {
		return false
	}
	for k, v := range uc {
		if k == PullingErrorKey {
			continue
		}
		if vv, ok := that[k]; !ok || vv != v {
			return false
		}
	}
	return true
}

const (
	LRUTimeout = 30 * time.Minute
)

var (
	config  *Config
	once    sync.Once
	Version = "debug"
)

func (g *Group) BuildMasterKeys() {
	servers := g.Servers
	for j := range servers {
		s := &servers[j]
		s.MasterKey = cipher.EVPBytesToKey(s.Password, cipher.CiphersConf[s.Method].KeyLen)
	}
}
func (g *Group) BuildUserContextPool(timeout time.Duration) {
	g.UserContextPool = (*UserContextPool)(lru.New(lru.FixedTimeout, int64(timeout)))
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
	logged := false
	for i := range config.Groups {
		g := &config.Groups[i]
		for j, upstreamConf := range g.Upstreams {
			var upstream Upstream
			switch upstreamConf["type"] {
			case "outline":
				var outline Outline
				err = Map2Upstream(upstreamConf, &outline)
				if err != nil {
					return
				}
				upstream = outline
			default:
				return fmt.Errorf("unknown upstream type: %v", upstreamConf["type"])
			}
			if !logged {
				log.Println("pulling configures from upstreams...")
				logged = true
			}
			servers, err := upstream.GetServers()
			if err != nil {
				var netError net.Error
				if errors.As(err, &netError) {
					upstreamConf[PullingErrorKey] = PullingErrorNetError
				}
				log.Printf("[warning] Failed to retrieve configure from groups[%d].upstreams[%d]: %v: %v\n", i, j, err, upstreamConf[PullingErrorKey])
				continue
			}
			for i := range servers {
				servers[i].UpstreamConf = &upstreamConf
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
	for i := range config.Groups {
		g := &config.Groups[i]
		g.BuildUserContextPool(LRUTimeout)
		g.BuildMasterKeys()
	}
}

func BuildConfig(confPath string) (conf *Config, err error) {
	conf = new(Config)
	conf.ConfPath = confPath
	b, err := os.ReadFile(confPath)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, conf); err != nil {
		return nil, err
	}
	if err = parseUpstreams(conf); err != nil {
		return nil, err
	}
	if err = check(conf); err != nil {
		return nil, err
	}
	build(conf)
	return
}

func SetConfig(conf *Config) {
	config = conf
}

func GetConfig() *Config {
	once.Do(func() {
		var err error

		version := flag.Bool("v", false, "version")
		confPath := flag.String("conf", "example.json", "config file path")
		flag.Parse()

		if *version {
			fmt.Println(Version)
			os.Exit(0)
		}

		if config, err = BuildConfig(*confPath); err != nil {
			log.Fatalln(err)
		}
	})
	return config
}

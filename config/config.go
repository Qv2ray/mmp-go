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
	Name         string        `json:"name"`
	Target       string        `json:"target"`
	Method       string        `json:"method"`
	Password     string        `json:"password"`
	MasterKey    []byte        `json:"-"`
	UpstreamConf *UpstreamConf `json:"-"`
}

type Group struct {
	Name            string           `json:"name"`
	Port            int              `json:"port"`
	Servers         []Server         `json:"servers"`
	Upstreams       []UpstreamConf   `json:"upstreams"`
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

func pullFromUpstream(upstream Upstream, upstreamConf *UpstreamConf) ([]Server, error) {
	servers, err := upstream.GetServers()
	if err != nil {
		return nil, err
	}
	for i := range servers {
		servers[i].UpstreamConf = upstreamConf
	}
	return servers, nil
}

func parseUpstreams(config *Config) (err error) {
	var wg sync.WaitGroup

	for _, g := range config.Groups {
		if len(g.Upstreams) > 0 {
			log.Println("Pulling from upstreams")
			break
		}
	}
	for i := range config.Groups {
		group := &config.Groups[i]
		mu := sync.Mutex{}
		for i, upstreamConf := range group.Upstreams {
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

			wg.Add(1)
			go func(group *Group, upstreamConf *UpstreamConf) {
				defer wg.Done()
				servers, err := pullFromUpstream(upstream, upstreamConf)
				if err != nil {
					if netError := new(net.Error); errors.As(err, netError) {
						(*upstreamConf)[PullingErrorKey] = PullingErrorNetError
					}
					log.Printf("[warning] Failed to pull from group %s upstream %s: %v\n", group.Name, upstream.GetName(), err)
					return
				}
				mu.Lock()
				group.Servers = append(group.Servers, servers...)
				mu.Unlock()
				log.Printf("Pulled %d servers from group %s upstream %s", len(servers), group.Name, upstream.GetName())
			}(group, &group.Upstreams[i])
		}
	}

	wg.Wait()
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
		suppressTimestamps := flag.Bool("suppress-timestamps", false, "do not include timestamps in log")
		flag.Parse()

		if *version {
			fmt.Println(Version)
			os.Exit(0)
		}

		if *suppressTimestamps {
			log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
		}

		if config, err = BuildConfig(*confPath); err != nil {
			log.Fatalln(err)
		}
	})
	return config
}

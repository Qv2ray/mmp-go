package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/infra/lru"
)

type Config struct {
	ConfPath   string       `json:"-"`
	HttpClient *http.Client `json:"-"`
	Groups     []Group      `json:"groups"`
}

type Server struct {
	Name         string        `json:"name"`
	Target       string        `json:"target"`
	TCPFastOpen  bool          `json:"TCPFastOpen"`
	Method       string        `json:"method"`
	Password     string        `json:"password"`
	MasterKey    []byte        `json:"-"`
	UpstreamConf *UpstreamConf `json:"-"`
}

type Group struct {
	Name                string           `json:"name"`
	Port                int              `json:"port"`
	ListenerTCPFastOpen bool             `json:"listenerTCPFastOpen"`
	Servers             []Server         `json:"servers"`
	Upstreams           []UpstreamConf   `json:"upstreams"`
	UserContextPool     *UserContextPool `json:"-"`

	// AuthTimeoutSec sets a TCP read timeout to drop connections that fail to finish auth in time.
	// Default: no timeout
	// outline-ss-server uses 59s, which is claimed to be the most common timeout for servers that do not respond to invalid requests.
	AuthTimeoutSec int `json:"authTimeoutSec"`

	// DrainOnAuthFail controls whether to fallback to the first server in the group when authentication fails.
	// Default: fallback to 1st server
	// Set to true to drain the connection when authentication fails.
	DrainOnAuthFail bool `json:"drainOnAuthFail"`
}

type UpstreamConf struct {
	Name         string          `json:"name"`
	Type         string          `json:"type"`
	Settings     json.RawMessage `json:"settings"`
	PullingError error           `json:"-"`
	Upstream     Upstream        `json:"-"`
}

func (uc UpstreamConf) Equal(that UpstreamConf) bool {
	return uc.Name == that.Name && uc.Type == that.Type && uc.Upstream.Equal(that.Upstream)
}

const (
	LRUTimeout = 30 * time.Minute
)

var (
	config  *Config
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

func pullFromUpstream(upstreamConf *UpstreamConf, c *http.Client) ([]Server, error) {
	servers, err := upstreamConf.Upstream.GetServers(c)
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

	for i := range config.Groups {
		group := &config.Groups[i]
		mu := sync.Mutex{}
		for i := range group.Upstreams {
			upstreamConf := &group.Upstreams[i]

			switch upstreamConf.Type {
			case "outline":
				upstreamConf.Upstream = &Outline{
					Name: upstreamConf.Name,
				}
				err = json.Unmarshal(upstreamConf.Settings, upstreamConf.Upstream)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown upstream type: %v", upstreamConf.Type)
			}

			wg.Add(1)
			go func(group *Group, upstreamConf *UpstreamConf) {
				defer wg.Done()
				servers, err := pullFromUpstream(upstreamConf, config.HttpClient)
				if err != nil {
					upstreamConf.PullingError = err
					log.Printf("[warning] Failed to pull from group %s upstream %s: %v\n", group.Name, upstreamConf.Name, err)
					return
				}
				mu.Lock()
				group.Servers = append(group.Servers, servers...)
				mu.Unlock()
				log.Printf("Pulled %d servers from group %s upstream %s\n", len(servers), group.Name, upstreamConf.Name)
			}(group, upstreamConf)
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

func BuildConfig(confPath string, c *http.Client) (conf *Config, err error) {
	conf = new(Config)
	conf.ConfPath = confPath
	conf.HttpClient = c
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

func NewConfig(c *http.Client) *Config {
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

	if config, err = BuildConfig(*confPath, c); err != nil {
		log.Fatalln(err)
	}
	return config
}

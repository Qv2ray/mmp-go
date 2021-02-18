package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Qv2ray/mmp-go/cipher"
	"github.com/Qv2ray/mmp-go/infra/lru"
	"log"
	"os"
	"sort"
	"sync"
	"time"
)

type Config struct {
	ConfPath string  `json:"-"`
	Groups   []Group `json:"groups"`
}
type Server struct {
	Target       string `json:"target"`
	Method       string `json:"method"`
	Password     string `json:"password"`
	MasterKey    []byte `json:"-"`
	UpstreamHash string `json:"-"`
}
type Group struct {
	Port            int              `json:"port"`
	Servers         []Server         `json:"servers"`
	Upstreams       []UpstreamConf   `json:"upstreams"`
	LRUSize         int              `json:"lruSize"`
	UserContextPool *UserContextPool `json:"-"`
}
type UpstreamConf struct {
	ConfItems    map[string]string
	PullingError error
}

func (uc *UpstreamConf) Hash() string {
	var kv [][2]string
	for k, v := range uc.ConfItems {
		if k == "" || v == "" {
			continue
		}
		kv = append(kv, [2]string{k, v})
	}
	if len(kv) == 0 {
		return ""
	}
	sort.Slice(kv, func(i, j int) bool {
		return kv[i][0] < kv[i][1]
	})
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.LittleEndian, &kv); err != nil {
		return ""
	}
	h := sha256.New()
	h.Write(buf.Bytes())
	return hex.EncodeToString(h.Sum(nil))
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
			switch upstreamConf.ConfItems["type"] {
			case "outline":
				var outline Outline
				err = Map2upstream(upstreamConf.ConfItems, &outline)
				if err != nil {
					return
				}
				upstream = outline
			default:
				return fmt.Errorf("unknown upstream type: %v", upstreamConf.ConfItems["type"])
			}
			if !logged {
				log.Println("pulling configures from upstreams...")
				logged = true
			}
			servers, err := upstream.GetServers()
			if err != nil {
				upstreamConf.PullingError = err
				log.Printf("[warning] Failed to retrieve configure from groups[%d].upstreams[%d]: %v\n", i, j, err)
				continue
			}
			upstreamHash := upstreamConf.Hash()
			for i := range servers {
				servers[i].UpstreamHash = upstreamHash
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

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
	"path"
	"sync"
	"syscall"
)

type Config struct {
	ConfPath       string  `json:"-"`
	Groups         []Group `json:"groups"`
	ClientCapacity int     `json:"clientCapacity"`
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
var DaemonMode bool

const (
	// program name
	Name = "mmp-go"
	// around 30kB per client if there are 300 servers to forward
	DefaultClientCapacity = 100
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
	logged := false
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
			if !logged {
				log.Println("pulling configures from upstreams...")
				logged = true
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
	globalClientCapacity := config.ClientCapacity
	if globalClientCapacity == 0 {
		globalClientCapacity = DefaultClientCapacity
	}
	for i := range config.Groups {
		g := &config.Groups[i]
		g.BuildUserContextPool(globalClientCapacity)
		g.BuildMasterKeys()
	}
}
func redirectOut(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stdin.Fd())); err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd())); err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd())); err != nil {
		return err
	}
	if file.Fd() > 2 {
		file.Close()
	}
	return err
}

func BuildConfig(confPath string) (conf *Config) {
	conf = new(Config)
	conf.ConfPath = confPath
	b, err := ioutil.ReadFile(confPath)
	if err != nil {
		log.Fatalln(err)
	}
	if err = json.Unmarshal(b, conf); err != nil {
		log.Fatalln(err)
	}
	if err = parseUpstreams(conf); err != nil {
		log.Fatalln(err)
	}
	if err = check(conf); err != nil {
		log.Fatalln(err)
	}
	build(conf)
	return
}

func SetConfig(conf *Config) {
	config = conf
}

func GetConfig() *Config {
	once.Do(func() {
		version := flag.Bool("v", false, "version")
		sig := flag.String("s", "", "signal: start/stop/reload")
		confPath := flag.String("conf", "example.json", "config file path")
		logPath := flag.String("log", "", "the file path to write log")
		flag.Parse()

		if *version {
			fmt.Println(Version)
			os.Exit(0)
		}
		if !DaemonMode {
			switch *sig {
			case "start":
				if !path.IsAbs(*confPath) {
					log.Fatalln("[error] daemon needs an absolute path of conf")
				}
				if err := start(); err != nil {
					log.Fatalln("[error]", err)
				}
				os.Exit(0)
			case "stop":
				if err := stop(); err != nil {
					log.Fatalln("[error]", err)
				}
				os.Exit(0)
			case "reload":
				if err := reload(); err != nil {
					log.Fatalln("[error]", err)
				}
				os.Exit(0)
			case "":
			default:
				log.Fatalln(fmt.Sprintf(`[error] invalid option: "-s %v"`, *sig))
			}
		}
		if *logPath != "" {
			err := redirectOut(*logPath)
			if err != nil {
				log.Fatalln(err)
			}
		}
		config = BuildConfig(*confPath)
	})
	return config
}

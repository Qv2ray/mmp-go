package main

import (
	"log"
	"net"
)

type MemoryServerAddr struct {
	TCPAddr *net.TCPAddr
	UDPAddr *net.UDPAddr
}

type MemoryConfig struct {
	ServerAddr     map[string]MemoryServerAddr
	PasswordServer map[[32]byte]string
}

func NewMemoryConfig(config *Config) *MemoryConfig {
	if config == nil {
		panic("nil config")
	}

	serverAddrMap := map[string]MemoryServerAddr{}
	passwordServerMap := map[[32]byte]string{}

	for serverName, server := range config.Servers {
		tcpAddr, err := net.ResolveTCPAddr("tcp", server.Addr)
		if err != nil {
			log.Fatalf("failed to resolve tcp address %s for %s: %v", server.Addr, serverName, err)
		}

		udpAddr, err := net.ResolveUDPAddr("udp", server.Addr)
		if err != nil {
			log.Fatalf("failed to resolve udp address %s for %s: %v", server.Addr, serverName, err)
		}

		serverAddrMap[serverName] = MemoryServerAddr{
			TCPAddr: tcpAddr,
			UDPAddr: udpAddr,
		}

		if len(server.Passwords) == 0 {
			log.Fatalf("server %s does have password!", serverName)
		}

		for _, password := range server.Passwords {
			passwordServerMap[kdf(password)] = serverName
		}
	}
	return &MemoryConfig{
		ServerAddr:     serverAddrMap,
		PasswordServer: passwordServerMap,
	}
}

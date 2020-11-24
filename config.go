package main

import (
	"github.com/go-yaml/yaml"
	"log"
	"os"
)

type ServerConfig struct {
	Addr      string   `json:"listen" yaml:"listen"`
	Method    string   `json:"method" yaml:"method"`
	Passwords []string `json:"passwords" yaml:"passwords"`
}

type Config struct {
	Listen  string                  `json:"addr" yaml:"addr"`
	Servers map[string]ServerConfig `json:"servers" yaml:"servers"`
}

func NewConfigFromYAMLFile(path string) *Config {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("failed to open file %s: %v", path, err)
	}

	cfg := Config{}
	err = yaml.NewDecoder(f).Decode(&cfg)
	if err != nil {
		log.Fatalf("failed to read yaml %s: %v", path, err)
	}

	return &cfg
}
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
	"net"
	"strconv"
)

type Outline struct {
	Type          string `json:"type"`
	Server        string `json:"server"`
	SSHPort       string `json:"sshPort"`
	SSHUsername   string `json:"sshUsername"`
	SSHPrivateKey string `json:"sshPrivateKey"`
	SSHPassword   string `json:"sshPassword"`
}

func (outline Outline) getConfig() ([]byte, error) {
	var (
		config      *ssh.ClientConfig
		authMethods []ssh.AuthMethod
	)
	if outline.SSHPrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(outline.SSHPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse privateKey error: %v", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	authMethods = append(authMethods, ssh.Password(outline.SSHPassword))
	username := outline.SSHUsername
	if username == "" {
		username = "root"
	}
	config = &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	port := outline.SSHPort
	if port == "" {
		port = "22"
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(outline.Server, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()
	out, err := session.CombinedOutput("cat /opt/outline/persisted-state/shadowbox_config.json")
	if err != nil {
		err = fmt.Errorf("%v: %v", string(bytes.TrimSpace(out)), err)
		return nil, err
	}
	return out, nil
}

func (outline Outline) GetServers() (servers []Server, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("outline.GetGroups: %v", err)
		}
	}()
	b, err := outline.getConfig()
	if err != nil {
		return
	}
	var config ShadowboxConfig
	err = json.Unmarshal(b, &config)
	if err != nil {
		return
	}
	return config.ToServers(outline.Server), nil
}

type AccessKey struct {
	ID               string `json:"id"`
	MetricsID        string `json:"metricsId"`
	Name             string `json:"name"`
	Password         string `json:"password"`
	Port             int    `json:"port"`
	EncryptionMethod string `json:"encryptionMethod"`
}

func (key *AccessKey) ToServer(host string) Server {
	return Server{
		Target:   net.JoinHostPort(host, strconv.Itoa(key.Port)),
		Method:   key.EncryptionMethod,
		Password: key.Password,
	}
}

type ShadowboxConfig struct {
	AccessKeys []AccessKey `json:"accessKeys"`
	NextID     int         `json:"nextId"`
}

func (config *ShadowboxConfig) ToServers(host string) []Server {
	var servers []Server
	for _, k := range config.AccessKeys {
		servers = append(servers, k.ToServer(host))
	}
	return servers
}

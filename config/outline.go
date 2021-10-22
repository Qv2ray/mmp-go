package config

import (
	"bytes"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type Outline struct {
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	Server                string `json:"server"`
	Link                  string `json:"link"`
	SSHPort               string `json:"sshPort"`
	SSHUsername           string `json:"sshUsername"`
	SSHPrivateKey         string `json:"sshPrivateKey"`
	SSHPassword           string `json:"sshPassword"`
	ApiUrl                string `json:"apiUrl"`
	ApiCertSha256         string `json:"apiCertSha256"`
	TCPFastOpen           bool   `json:"TCPFastOpen"`
	AccessKeyPortOverride int    `json:"accessKeyPortOverride"`
}

const timeout = 10 * time.Second

var IncorrectPasswordErr = fmt.Errorf("incorrect password")

func (outline Outline) getConfig() ([]byte, error) {
	if outline.Server == "" {
		return nil, fmt.Errorf("server field cannot be empty")
	}
	tryList := []func() ([]byte, error){
		outline.getConfigFromLink,
		outline.getConfigFromApi,
		outline.getConfigFromSSH,
	}
	var (
		err  error
		errs []error
		b    []byte
	)
	for _, f := range tryList {
		b, err = f()
		if err != nil {
			// try next func
			b = nil
			errs = append(errs, err)
			continue
		}
		if b != nil {
			// valid result, break
			break
		}
	}
	if b != nil {
		// valid result
		return b, nil
	}
	if len(errs) > 0 {
		// concatenate errors
		err = errs[0]
		for i := 1; i < len(errs); i++ {
			err = fmt.Errorf("%w; %s", err, errs[i].Error())
		}
		return nil, err
	}
	// b and err is both nil, no valid info to get configure
	return nil, InvalidUpstreamErr
}

func (outline Outline) getConfigFromLink() ([]byte, error) {
	if outline.Link == "" {
		return nil, nil
	}
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(outline.Link)
	if err != nil {
		return nil, fmt.Errorf("getConfigFromLink failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (outline Outline) getConfigFromApi() ([]byte, error) {
	if outline.ApiUrl == "" || outline.ApiCertSha256 == "" {
		return nil, nil
	}
	client := http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				h := crypto.SHA256.New()
				for _, line := range rawCerts {
					h.Write(line)
				}
				fingerprint := hex.EncodeToString(h.Sum(nil))
				if !strings.EqualFold(fingerprint, outline.ApiCertSha256) {
					return fmt.Errorf("incorrect certSha256 from server: %v", strings.ToUpper(fingerprint))
				}
				return nil
			},
		}},
		Timeout: timeout,
	}
	outline.ApiUrl = strings.TrimSuffix(outline.ApiUrl, "/")
	resp, err := client.Get(fmt.Sprintf("%v/access-keys", outline.ApiUrl))
	if err != nil {
		return nil, fmt.Errorf("getConfigFromApi failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (outline Outline) getConfigFromSSH() ([]byte, error) {
	if outline.SSHUsername == "" || (outline.SSHPrivateKey == "" && outline.SSHPassword == "") {
		return nil, nil
	}
	var (
		conf        *ssh.ClientConfig
		authMethods []ssh.AuthMethod
	)
	if outline.SSHPrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(outline.SSHPrivateKey))
		if err != nil {
			return nil, fmt.Errorf("parse privateKey error: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	authMethods = append(authMethods, ssh.Password(outline.SSHPassword))
	username := outline.SSHUsername
	if username == "" {
		username = "root"
	}
	conf = &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	port := outline.SSHPort
	if port == "" {
		port = "22"
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort(outline.Server, port), conf)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	const cmd = "cat /opt/outline/persisted-state/shadowbox_config.json"
	gid, err := getGroupID(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get gid: %w", err)
	}
	if gid == "0" {
		session, err := client.NewSession()
		if err != nil {
			return nil, fmt.Errorf("failed to create session: %w", err)
		}
		defer session.Close()
		out, err := session.CombinedOutput(cmd)
		if err != nil {
			err = fmt.Errorf("%v: %w", string(bytes.TrimSpace(out)), err)
			return nil, err
		}
		return out, nil
	} else {
		out, err := sudoCombinedOutput(client, outline.SSHPassword, cmd)
		if err != nil {
			return nil, fmt.Errorf("failed to execute sudo: %w", err)
		}
		return out, nil
	}
}

func getGroupID(client *ssh.Client) (gid string, err error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	b, err := session.Output("id -g")
	return strings.TrimSpace(string(b)), err
}

func sudoCombinedOutput(client *ssh.Client, password string, cmd string) (b []byte, err error) {
	const prompt = "[INPUT YOUR PASSWORD]"
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	b, err = session.CombinedOutput("sh -c " + strconv.Quote(fmt.Sprintf("echo %v|sudo -p %v -S %v", strconv.Quote(password), strconv.Quote(prompt), cmd)))
	b = bytes.TrimPrefix(bytes.TrimSpace(b), []byte(prompt))
	if bytes.Contains(b, []byte(prompt)) {
		return b, IncorrectPasswordErr
	}
	return b, err
}

func (outline Outline) GetName() string {
	return outline.Name
}

func (outline Outline) GetServers() (servers []Server, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("outline.GetGroups: %w", err)
		}
	}()
	b, err := outline.getConfig()
	if err != nil {
		return
	}
	var conf ShadowboxConfig
	err = json.Unmarshal(b, &conf)
	if err != nil {
		return
	}
	return conf.ToServers(outline.Name, outline.Server, outline.TCPFastOpen, outline.AccessKeyPortOverride), nil
}

type AccessKey struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Password         string `json:"password"`
	Port             int    `json:"port"`
	EncryptionMethod string `json:"encryptionMethod"`
	Method           string `json:"method"` // the alias of EncryptionMethod
}

func (key *AccessKey) ToServer(name, host string, tfo bool, portOverride int) Server {
	method := key.EncryptionMethod
	if method == "" {
		method = key.Method
	}
	if portOverride == 0 {
		portOverride = key.Port
	}
	return Server{
		Name:        fmt.Sprintf("%s - %s", name, key.Name),
		Target:      net.JoinHostPort(host, strconv.Itoa(portOverride)),
		TCPFastOpen: tfo,
		Method:      method,
		Password:    key.Password,
	}
}

type ShadowboxConfig struct {
	AccessKeys []AccessKey `json:"accessKeys"`
}

func (c *ShadowboxConfig) ToServers(name, host string, tfo bool, portOverride int) []Server {
	var servers []Server
	for _, k := range c.AccessKeys {
		servers = append(servers, k.ToServer(name, host, tfo, portOverride))
	}
	return servers
}

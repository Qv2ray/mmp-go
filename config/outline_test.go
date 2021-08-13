package config

import (
	"golang.org/x/crypto/ssh"
	"net"
	"testing"
)

func TestSudoOutput(t *testing.T) {
	username := "testuser"
	password := "testpassword"
	conf := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("localhost", "22"), conf)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	b, err := sudoCombinedOutput(client, password, "echo ok")
	if err != nil {
		t.Fatal(string(b), err)
	}
	b, err = sudoCombinedOutput(client, "fakepassword", "echo ok")
	if err != nil {
		t.Log(string(b), err)
	}
}

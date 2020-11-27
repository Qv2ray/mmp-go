package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"github.com/mzz2017/glen/common/linklist"
	"golang.org/x/crypto/hkdf"
	"io"
	"log"
	"net"
	"syscall"
	"time"
)

func ListenTCP(group *Group) (err error) {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", group.Port))
	if err != nil {
		return
	}
	defer l.Close()
	for {
		conn, err := l.Accept()
		if err != nil {
			continue
		}
		go func() {
			err := handleConn(conn, group)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

type UserContext linklist.Linklist

func NewUserContext(servers []Server) *UserContext {
	ctx := linklist.NewLinklist()
	for i := range servers {
		ctx.PushBack(&servers[i])
	}
	return (*UserContext)(ctx)
}

func (ctx *UserContext) Infra() *linklist.Linklist {
	return (*linklist.Linklist)(ctx)
}

func handleConn(conn net.Conn, group *Group) error {
	/*
	   https://github.com/shadowsocks/shadowsocks-org/blob/master/whitepaper/whitepaper.md
	*/
	defer conn.Close()
	//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
	var buf [32 + 2 + 16]byte
	n, err := io.ReadFull(conn, buf[:])
	if err != nil {
		return fmt.Errorf("handleConn readfull error: %v", err)
	}
	userIdent := conn.RemoteAddr().String()
	var userContext *UserContext
	nodeUserContext := group.UserContext.Get(userIdent)
	if nodeUserContext == nil {
		userContext = NewUserContext(group.Servers)
		group.UserContext.Insert(userIdent, userContext)
	} else {
		userContext = nodeUserContext.Val.(*UserContext)
	}
	server, err := auth(buf[:], userContext)
	if err != nil {
		log.Fatalln(fmt.Errorf("handleConn auth error: %v", err))
	}
	if server == nil {
		return nil
	}
	rc, err := net.Dial("tcp", server.Target)
	if err != nil {
		return fmt.Errorf("handleConn dial error: %v", err)
	}
	_, err = rc.Write(buf[:n])
	if err != nil {
		return fmt.Errorf("handleConn write error: %v", err)
	}
	if err := relay(conn, rc); !errors.Is(err, syscall.ETIMEDOUT) {
		return fmt.Errorf("handleConn relay error: %v", err)
	}
	return nil
}

func relay(lc, rc net.Conn) error {
	defer rc.Close()
	ch := make(chan error)
	go func() {
		_, err := io.Copy(lc, rc)
		lc.SetDeadline(time.Now())
		rc.SetDeadline(time.Now())
		ch <- err
	}()
	_, err := io.Copy(rc, lc)
	lc.SetDeadline(time.Now())
	rc.SetDeadline(time.Now())
	if err != nil {
		return err
	}
	return <-ch
}

func auth(data []byte, userContext *UserContext) (hit *Server, err error) {
	if len(data) < 50 {
		return nil, fmt.Errorf("length of data should be no less than 50")
	}
	ctx := userContext.Infra()
	for serverNode := ctx.Front(); serverNode != ctx.Tail(); serverNode = serverNode.Next() {
		server := serverNode.Val.(*Server)
		//d := make([]byte, len(data))
		//copy(d, data)
		if probe(data, server) {
			ctx.Promote(serverNode)
			return server, nil
		}
	}
	return nil, nil
}

func probe(data []byte, server *Server) bool {
	//[salt][encrypted payload length][length tag][encrypted payload][payload tag]
	conf := CiphersConf[server.Method]

	salt := data[:conf.SaltLen]
	cipherText := data[conf.SaltLen : conf.SaltLen+2+conf.TagLen]

	subKey := make([]byte, conf.KeyLen)
	kdf := hkdf.New(
		sha1.New,
		server.MasterKey,
		salt,
		[]byte("ss-subkey"),
	)
	io.ReadFull(kdf, subKey)

	nonce := make([]byte, conf.NonceLen) // equals to zero

	cipher, _ := conf.NewCipher(subKey)
	buf := make([]byte, 2)
	_, err := cipher.Open(buf, nonce, cipherText, nil)
	return err == nil
}

package main

import (
	"crypto/md5"
	"crypto/sha1"
	"flag"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"io"
	"io/ioutil"
	"log"
	"net"
)

// chacha20-ietf-poly1305
// salt(32)+len(2)+tag(16)
const ChaCha20SaltLen = 32
const ChaCha20KeyLen = 32
const ChaCha20TagLen = 16
const LenLen = 2
const ChaCha20BufferLen = ChaCha20SaltLen + LenLen + ChaCha20TagLen

func kdf(password string) (k [32]byte) {
	var b []byte
	h := md5.New()
	h.Write([]byte(password))
	h.Sum(b)
	copy(k[:], b)
	return
}

var configFilePath = flag.String("config", "config.yaml", "path to yaml configuration file")

func init() {
	flag.Parse()
}

func main() {
	config := NewConfigFromYAMLFile(*configFilePath)
	memConf := NewMemoryConfig(config)

	listener, err := net.Listen("tcp", ":4445")
	if err != nil {
		log.Fatalf("failed to listen tcp: %v", err)
	}
	defer listener.Close()
	log.Printf("started listening on %v", listener.Addr())

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept: %v", err)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()
			buf := make([]byte, ChaCha20BufferLen)
			nRecv, err := io.ReadFull(conn, buf)
			if nRecv == 0 {
				return
			}
			if err != nil {
				log.Printf("only %d of %d bytes read, draining", nRecv, ChaCha20BufferLen)
				_, _ = io.Copy(ioutil.Discard, conn)
				return
			}

			salt := buf[:ChaCha20SaltLen]
			subKeyBuf := make([]byte, ChaCha20KeyLen)
			lenBuf := make([]byte, LenLen)
			nonceBuf := make([]byte, 12) // Zero Nonce
			for password, serverName := range memConf.PasswordServer {
				r := hkdf.New(sha1.New, password[:], salt, []byte("ss-subkey"))
				_, _ = io.ReadFull(r, subKeyBuf)

				blk, _ := chacha20poly1305.New(subKeyBuf)
				_, err := blk.Open(lenBuf, nonceBuf, buf[ChaCha20SaltLen:], nil)
				if err == nil {
					log.Printf("relaying %v to %s...", conn.RemoteAddr(), serverName)
					upstream, err := net.DialTCP("tcp", nil, memConf.ServerAddr[serverName].TCPAddr)
					if err != nil {
						log.Printf("failed to dial to server %s: %v", serverName, err)
						return
					}
					_, err = upstream.Write(buf)
					go func() {
						_, _ = io.Copy(conn, upstream)
						_ = upstream.Close()
					}()
					_, _ = io.Copy(upstream, conn)
					_ = upstream.Close()
				}
			}
			log.Printf("failed to find a match for %v, draining", conn.RemoteAddr())
			_, _ = io.Copy(ioutil.Discard, conn)
		}(conn)
	}
}

package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha1"
	"github.com/Qv2ray/mmp-go/common/leakybuf"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"io"
)

var CiphersConf = map[string]CipherConf{
	"chacha20-ietf-poly1305": {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: chacha20poly1305.New},
	"chacha20-poly1305":      {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: chacha20poly1305.New},
	"aes-256-gcm":            {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: NewGcm},
	"aes-128-gcm":            {KeyLen: 16, SaltLen: 16, NonceLen: 12, TagLen: 16, NewCipher: NewGcm},
}

type CipherConf struct {
	KeyLen    int
	SaltLen   int
	NonceLen  int
	TagLen    int
	NewCipher func(key []byte) (cipher.AEAD, error)
}

const TCPMaxTagSize = 16
const MaxNonceSize = 12

var ZeroNonce [MaxNonceSize]byte
var TCPReuseDstBuf [2 + TCPMaxTagSize]byte

func (conf *CipherConf) Verify(masterKey []byte, salt []byte, cipherText []byte) bool {
	subKey := leakybuf.Get(conf.KeyLen)
	defer leakybuf.Put(subKey)
	kdf := hkdf.New(
		sha1.New,
		masterKey,
		salt,
		[]byte("ss-subkey"),
	)
	io.ReadFull(kdf, subKey)

	ciph, _ := conf.NewCipher(subKey)

	var buf []byte
	if len(cipherText) == 2+ciph.Overhead() {
		// TCP
		buf = TCPReuseDstBuf[:]
	} else {
		// UDP
		buf = leakybuf.Get(leakybuf.UDPBufSize)
		defer leakybuf.Put(buf)
	}
	_, err := ciph.Open(buf, ZeroNonce[:conf.NonceLen], cipherText, nil)
	return err == nil
}

func MD5Sum(d []byte) []byte {
	h := md5.New()
	h.Write(d)
	return h.Sum(nil)
}

func EVPBytesToKey(password string, keyLen int) (key []byte) {
	const md5Len = 16

	cnt := (keyLen-1)/md5Len + 1
	m := make([]byte, cnt*md5Len)
	copy(m, MD5Sum([]byte(password)))

	// Repeatedly call md5 until bytes generated is enough.
	// Each call to md5 uses data: prev md5 sum + password.
	d := make([]byte, md5Len+len(password))
	start := 0
	for i := 1; i < cnt; i++ {
		start += md5Len
		copy(d, m[start-md5Len:start])
		copy(d[md5Len:], password)
		copy(m[start:], MD5Sum(d))
	}
	return m[:keyLen]
}

func NewGcm(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

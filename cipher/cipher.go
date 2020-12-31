package cipher

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha1"
	"github.com/Qv2ray/mmp-go/common/pool"
	"github.com/studentmain/smaead"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"io"
)

type CipherConf struct {
	KeyLen           int
	SaltLen          int
	NonceLen         int
	TagLen           int
	NewCipher        func(key []byte) (cipher.AEAD, error)
	NewPartialCipher func(key []byte) (smaead.PartialAEAD, error)
}

const (
	MaxNonceSize = 12
	ATypeIPv4    = 1
	ATypeDomain  = 3
	ATypeIpv6    = 4
)

var (
	CiphersConf = map[string]CipherConf{
		"chacha20-ietf-poly1305": {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: chacha20poly1305.New, NewPartialCipher: NewPC20P1305},
		"chacha20-poly1305":      {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: chacha20poly1305.New, NewPartialCipher: NewPC20P1305},
		"aes-256-gcm":            {KeyLen: 32, SaltLen: 32, NonceLen: 12, TagLen: 16, NewCipher: NewGcm, NewPartialCipher: NewPGcm},
		"aes-128-gcm":            {KeyLen: 16, SaltLen: 16, NonceLen: 12, TagLen: 16, NewCipher: NewGcm, NewPartialCipher: NewPGcm},
	}
	ZeroNonce  [MaxNonceSize]byte
	ReusedInfo = []byte("ss-subkey")
)

func (conf *CipherConf) Verify(buf []byte, masterKey []byte, salt []byte, cipherText []byte) ([]byte, bool) {
	subKey := pool.Get(conf.KeyLen)
	defer pool.Put(subKey)
	kdf := hkdf.New(
		sha1.New,
		masterKey,
		salt,
		ReusedInfo,
	)
	io.ReadFull(kdf, subKey)

	ciph, _ := conf.NewCipher(subKey)

	if _, err := ciph.Open(buf[:0], ZeroNonce[:conf.NonceLen], cipherText, nil); err != nil {
		return nil, false
	}
	return buf[:len(cipherText)-ciph.Overhead()], true
}

func (conf *CipherConf) UnsafeVerifyATyp(buf []byte,masterKey []byte, salt []byte, cipherText []byte) bool {
	subKey := pool.Get(conf.KeyLen)
	defer pool.Put(subKey)
	kdf := hkdf.New(
		sha1.New,
		masterKey,
		salt,
		ReusedInfo,
	)
	io.ReadFull(kdf, subKey)

	ciph, _ := conf.NewPartialCipher(subKey)
	plain := ciph.OpenWithoutCheck(buf[:0], ZeroNonce[:conf.NonceLen], cipherText[:1])
	atyp := plain[0]
	switch atyp {
	case ATypeIPv4, ATypeDomain, ATypeIpv6:
		return true
	}
	return false
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

func NewPC20P1305(key []byte) (smaead.PartialAEAD, error) {
	return smaead.NewPartialChacha20Poly1305(key)
}

func NewPGcm(key []byte) (smaead.PartialAEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return smaead.NewPartialGCM(block)
}

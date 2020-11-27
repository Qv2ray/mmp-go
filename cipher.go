package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
)

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
		panic(err.Error())
	}
	return cipher.NewGCM(block)
}
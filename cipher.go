package main

import (
	"crypto/md5"
)

func EVPBytesToKey(passwd []byte) []byte {
	h := md5.New()
	h.Write(passwd)
	b := make([]byte, 32)
	copy(b, h.Sum(nil))
	return b
}

func NewGcm() {

}
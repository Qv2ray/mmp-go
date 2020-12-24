package leakybuf

import (
	"math/bits"
	"sync"
)

var pool = make(map[int]chan []byte)
var mu sync.RWMutex

const maxPoolSize = 256
const UDPBufSize = 64 * 1024
const maxSize = 1 << 17

func getClosestSize(need int) (size int) {
	// if need is exactly 2^n, return it
	if need&(need-1) == 0 {
		return need
	}
	// or return its closest 2^n
	return 1 << bits.Len(uint(need))
}

func Get(need int) []byte {
	if need > maxSize {
		return make([]byte, need)
	}
	size := getClosestSize(need)
	mu.RLock()
	c, ok := pool[size]
	if !ok {
		mu.RUnlock()
		mu.Lock()
		if c, ok = pool[size]; !ok {
			pool[size] = make(chan []byte, maxPoolSize)
			mu.Unlock()
			return make([]byte, need, size)
		}
		mu.Unlock()
	} else {
		mu.RUnlock()
	}
	select {
	case buf := <-c:
		return buf[:need]
	default:
	}
	return make([]byte, need, size)
}

func Put(buf []byte) {
	size := cap(buf)
	if size > maxSize {
		return
	}
	mu.RLock()
	c, ok := pool[size]
	if ok {
		mu.RUnlock()
		select {
		case c <- buf[:size]:
		default:
		}
	} else {
		mu.RUnlock()
		mu.Lock()
		if c, ok = pool[size]; !ok {
			pool[size] = make(chan []byte, maxPoolSize)
			mu.Unlock()
		} else {
			mu.Unlock()
		}
		select {
		case pool[size] <- buf[:size]:
		default:
		}
	}
}

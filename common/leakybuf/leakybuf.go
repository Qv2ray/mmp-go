package leakybuf

import (
	"sync"
)

var pool = make(map[int]chan []byte)
var mu sync.RWMutex

const maxPoolSize = 1024
const UDPBufSize = 64 * 1024
const maxSize = UDPBufSize

func Get(size int) []byte {
	if size > maxSize {
		return make([]byte, size)
	}
	mu.RLock()
	c, ok := pool[size]
	if !ok {
		mu.RUnlock()
		mu.Lock()
		if c, ok = pool[size]; !ok {
			pool[size] = make(chan []byte, maxPoolSize)
			mu.Unlock()
			return make([]byte, size)
		}
		mu.Unlock()
	} else {
		mu.RUnlock()
	}
	select {
	case buf := <-c:
		return buf[:size]
	default:
	}
	return make([]byte, size)
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

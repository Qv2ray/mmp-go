package leakybuf

var pool = make(map[int]chan []byte)

const maxsize = 1024

const UDPBufSize = 64 * 1024

func Get(size int) []byte {
	c, ok := pool[size]
	if !ok {
		pool[size] = make(chan []byte, maxsize)
		return make([]byte, size)
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
	c, ok := pool[size]
	if ok {
		select {
		case c <- buf[:size]:
		default:
		}
	} else {
		pool[size] = make(chan []byte, maxsize)
		select {
		case pool[size] <- buf[:size]:
		default:
		}
	}
}

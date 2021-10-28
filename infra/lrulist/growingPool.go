package lrulist

import (
	"math/bits"
	"sync"
)

type growingPool struct {
	pool *sync.Pool
	size int
	mu   sync.Mutex
}

func newGrowingPool(size int) *growingPool {
	return &growingPool{
		pool: &sync.Pool{New: func() interface{} {
			return make([]*Node, size)
		}},
		size: size,
	}
}

func (p *growingPool) Get(need int) []*Node {
	p.mu.Lock()
	defer p.mu.Unlock()
	if need > p.size {
		p.size = 1 << bits.Len32(uint32(need))
		p.pool = &sync.Pool{New: func() interface{} {
			return make([]*Node, p.size)
		}}
	}
	return p.pool.Get().([]*Node)[:need]
}

func (p *growingPool) Put(l []*Node) {
	if cap(l) != p.size {
		return
	}
	p.pool.Put(l[:p.size])
	return
}

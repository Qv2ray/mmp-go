package lru

import (
	"github.com/Qv2ray/shadomplexer-go/common/linklist"
	"sync"
)

type LRU struct {
	list         *linklist.Linklist
	index        map[interface{}]*linklist.Node
	reverseIndex map[*linklist.Node]interface{}
	sync.Mutex
	maxLen int
}

func New(maxLen int) *LRU {
	return &LRU{
		index:  make(map[interface{}]*linklist.Node),
		reverseIndex:  make(map[*linklist.Node]interface{}),
		maxLen: maxLen,
		list: linklist.NewLinklist(),
	}
}

func (l *LRU) Get(key interface{}) *linklist.Node {
	l.Lock()
	defer l.Unlock()
	v, ok := l.index[key]
	if !ok {
		return nil
	}
	l.list.Promote(v)
	return v
}

func (l *LRU) Insert(key interface{}, val interface{}) {
	l.Lock()
	defer l.Unlock()
	node := l.list.PushFront(val)
	l.index[key] = node
	l.reverseIndex[node] = key
	if len(l.index) > l.maxLen {
		back := l.list.Back()
		key := l.reverseIndex[back]
		l.list.Remove(back)
		delete(l.index, key)
		delete(l.reverseIndex, back)
	}
}

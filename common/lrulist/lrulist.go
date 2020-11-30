package lrulist

import (
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	Val    interface{}
	weight uint32
}

type LruList struct {
	list           []*Node
	muList         sync.Mutex
	c              *time.Ticker
	avg            uint32
	max            uint32
	insertStrategy InsertStrategy
}

type InsertStrategy int

const (
	InsertToFront InsertStrategy = iota
	InsertToAverage
)

func New(updateInterval time.Duration, insertStrategy InsertStrategy) *LruList {
	lru := &LruList{
		insertStrategy: insertStrategy,
		c:              time.NewTicker(updateInterval),
	}
	go lru.updater()
	return lru
}

func NewWithList(updateInterval time.Duration, insertStrategy InsertStrategy, list []interface{}) *LruList {
	l := make([]*Node, len(list), 2*len(list))
	for i := range list {
		l[i] = &Node{Val: list[i]}
	}
	lru := New(updateInterval, insertStrategy)
	lru.list = l
	return lru
}

func (l *LruList) Close() (err error) {
	l.c.Stop()
	return nil
}

func (l *LruList) GetListCopy() []*Node {
	list := make([]*Node, len(l.list))
	l.muList.Lock()
	copy(list, l.list)
	l.muList.Unlock()
	return list
}

// promote but lazy sort
func (l *LruList) Promote(node *Node) {
	atomic.AddUint32(&node.weight, 1)
}

func (l *LruList) Insert(val interface{}) *Node {
	node := &Node{Val: val}
	if l.insertStrategy == InsertToFront {
		node.weight = l.max + 1
	} else {
		node.weight = l.avg + 1
	}
	l.muList.Lock()
	defer l.muList.Unlock()
	var insertBefore int
	for i := range l.list {
		if l.list[i].weight < node.weight {
			insertBefore = i
			break
		}
	}
	if cap(l.list) > len(l.list) {
		l.list = l.list[:len(l.list)+1]
		for i := len(l.list) - 1; i > insertBefore; i-- {
			l.list[i] = l.list[i-1]
		}
		l.list[insertBefore] = node
	} else {
		list := make([]*Node, len(l.list)+1, 2*len(l.list))
		for i := len(l.list) - 1; i >= insertBefore; i-- {
			list[i+1] = l.list[i]
		}
		list[insertBefore] = node
		copy(list[:insertBefore], l.list)
		l.list = list
	}
	return node
}

// O(n) scan
func (l *LruList) Remove(node *Node) {
	l.muList.Lock()
	defer l.muList.Unlock()
	targetIndex := -1
	for i := range l.list {
		if l.list[i] == node {
			targetIndex = i
			break
		}
	}
	if targetIndex == -1 {
		return
	}
	for i := targetIndex; i < len(l.list)-1; i++ {
		l.list[i] = l.list[i+1]
	}
	l.list = l.list[:len(l.list)-1]
}

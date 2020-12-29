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
	updateTicker   *time.Ticker
	avg            uint32
	max            uint32
	insertStrategy InsertStrategy
	pool           *growingPool
}

type InsertStrategy int

const (
	InsertFront InsertStrategy = iota
	InsertAverage
)

func New(updateInterval time.Duration, insertStrategy InsertStrategy) *LruList {
	list := &LruList{
		insertStrategy: insertStrategy,
		updateTicker:   time.NewTicker(updateInterval),
		pool:           newGrowingPool(1),
	}
	list.list = list.pool.Get(1)
	go list.updater()
	return list
}

func NewWithList(updateInterval time.Duration, insertStrategy InsertStrategy, list []interface{}) *LruList {
	lruList := &LruList{
		insertStrategy: insertStrategy,
		updateTicker:   time.NewTicker(updateInterval),
		pool:           newGrowingPool(len(list)),
	}
	l := lruList.pool.Get(len(list))
	for i := range list {
		l[i] = &Node{Val: list[i]}
	}
	lruList.list = l
	return lruList
}

func (l *LruList) Close() (err error) {
	l.updateTicker.Stop()
	return nil
}

// GetListCopy should be called when you want to traverse the list
func (l *LruList) GetListCopy() []*Node {
	l.muList.Lock()
	list := l.pool.Get(len(l.list))
	copy(list, l.list)
	l.muList.Unlock()
	return list
}

// GiveBackListCopy should be called when the list copy is no longer used
func (l *LruList) GiveBackListCopy(list []*Node) {
	l.pool.Put(list)
}

// promote but lazy sort. O(1)
func (l *LruList) Promote(node *Node) {
	atomic.AddUint32(&node.weight, 1)
}

// spend O(n) time to insert
func (l *LruList) Insert(val interface{}) *Node {
	node := &Node{Val: val}
	if l.insertStrategy == InsertFront {
		node.weight = l.max + 1
	} else {
		node.weight = l.avg + 1
	}
	l.muList.Lock()
	defer l.muList.Unlock()

	// insert into a roughly right position
	insertBefore := len(l.list)
	for i := range l.list {
		if l.list[i].weight < node.weight {
			insertBefore = i
			break
		}
	}

	// expand the list
	if cap(l.list) > len(l.list) {
		l.list = l.list[:len(l.list)+1]
		for i := len(l.list) - 1; i > insertBefore; i-- {
			l.list[i] = l.list[i-1]
		}
		l.list[insertBefore] = node
	} else {
		list := l.pool.Get(len(l.list) + 1)
		for i := len(l.list) - 1; i >= insertBefore; i-- {
			list[i+1] = l.list[i]
		}
		list[insertBefore] = node
		copy(list[:insertBefore], l.list[:insertBefore])
		l.pool.Put(l.list)
		l.list = list
	}
	return node
}

// O(n) scan to remove a node
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

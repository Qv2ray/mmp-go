package lrulist

import (
	"sort"
)

type SortableList []*Node

func (l SortableList) Len() int {
	return len(l)
}
func (l SortableList) Less(i, j int) bool {
	return l[i].weight > l[j].weight
}
func (l SortableList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l *LruList) updater() {
	// every interval time, updater divide counts by 2
	for range l.c.C {
		l.muList.Lock()
		sort.SliceStable(l.list, func(i, j int) bool {
			return l.list[i].weight > l.list[j].weight
		})
		l.avg = 0
		l.max = 0
		var sum uint64
		var cnt uint64
		for i := range l.list {
			if l.list[i].weight == 0 {
				break
			}
			cnt++
			if l.list[i].weight > l.max {
				l.max = l.list[i].weight
			}
			sum += uint64(l.list[i].weight)
			l.list[i].weight /= 2
		}
		if cnt != 0 {
			l.avg = uint32(sum / cnt)
		}
		l.muList.Unlock()
	}
}

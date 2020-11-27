package main

import "sync"

func main() {
	config := GetConfig()
	var wg sync.WaitGroup
	for i := range config.Groups {
		wg.Add(1)
		go func(group *Group) {
			ListenTCP(group)
			wg.Done()
		}(&config.Groups[i])
	}
	wg.Wait()
}

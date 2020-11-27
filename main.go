package main

import (
	"log"
	"sync"
)

func main() {
	config := GetConfig()
	var wg sync.WaitGroup
	for i := range config.Groups {
		wg.Add(1)
		go func(group *Group) {
			err := ListenTCP(group)
			if err != nil {
				log.Fatalln(err)
			}
			wg.Done()
		}(&config.Groups[i])
	}
	wg.Wait()
}

package main

import "log"

// not support windows
func signalHandler() {
	log.Println(`Signal-triggered configuration reloading is not supported on Windows`)
}

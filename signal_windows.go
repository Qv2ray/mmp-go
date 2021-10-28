package main

import (
	"log"

	"github.com/Qv2ray/mmp-go/config"
)

// not support windows
func signalHandler(*config.Config) {
	log.Println(`Signal-triggered configuration reloading is not supported on Windows`)
}

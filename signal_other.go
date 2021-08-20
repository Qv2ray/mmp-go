//go:build !windows
// +build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func signalHandler() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	for range ch {
		ReloadConfig()
	}
}

// +build !linux

package config

import (
	"os"
)

func start() error {
	return nil
}
func stop() error {
	return nil
}
func reload() error {
	return nil
}

func redirectOut(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	os.Stdout = file
	os.Stdin = file
	os.Stderr = file
	if file.Fd() > 2 {
		file.Close()
	}
	return err
}

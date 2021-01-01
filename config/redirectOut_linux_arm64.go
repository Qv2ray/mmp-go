package config

import (
	"log"
	"os"
)

func redirectOut(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	log.SetOutput(file)
	return err
}

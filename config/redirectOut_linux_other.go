// +build linux
// +build !arm64,!riscv64

package config

import (
	"os"
	"syscall"
)

func redirectOut(path string) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stdin.Fd())); err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stdout.Fd())); err != nil {
		return err
	}
	if err = syscall.Dup2(int(file.Fd()), int(os.Stderr.Fd())); err != nil {
		return err
	}
	if file.Fd() > 2 {
		file.Close()
	}
	return err
}

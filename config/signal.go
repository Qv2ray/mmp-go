package config

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/VividCortex/godaemon"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	if os.Args[0] == "[daemon]" {
		// init daemon
		err := daemonInit()
		if err != nil {
			log.Fatalln(err)
		}
		// init config
		_ = GetConfig()
	}
}

func readPID() (pid int, err error) {
	if runtime.GOOS != "linux" {
		return -1, fmt.Errorf("daemon only support linux")
	}
	b, err := ioutil.ReadFile(path.Join("/run/", Name+".pid"))
	if err != nil {
		return
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// pass pname="" if not check process name
func ProcessExists(pid int, pname string) bool {
	f, err := os.Open(path.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return false
	}
	defer f.Close()
	if pname == "" {
		return true
	}
	r := bufio.NewReader(f)
	var line []byte
	for err == nil {
		line, _, err = r.ReadLine()
		fields := bytes.Fields(line)
		if len(fields) < 2 || bytes.EqualFold(fields[0], []byte("Name")) {
			continue
		}
		return bytes.Equal(fields[1], []byte(pname))
	}
	return false
}

func daemonInit() (err error) {
	DaemonMode = true

	err = writePIDFile()
	if err != nil {
		return
	}
	return nil
}

func writePIDFile() (err error) {
	fd, err := syscall.Creat(path.Join("/run/", Name+".pid"),
		syscall.S_IRUSR|syscall.S_IWUSR|syscall.S_IRGRP|syscall.S_IROTH)
	if err != nil {
		return fmt.Errorf("syscall.Creat: %v", err)
	}
	_, err = syscall.Write(fd, []byte(strconv.Itoa(os.Getpid())))
	if err != nil {
		fmt.Errorf("syscall.Write: %v", err)
	}
	return nil
}

// https://github.com/golang/go/issues/15538
// We can not safely syscall.Fork in a multi-threaded program.
func fork() (err error) {
	_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{ProgramName: "[daemon]", CaptureOutput: false})
	return
}

func start() error {
	if pid, err := readPID(); err != nil && !os.IsNotExist(err) {
		return err
	} else if ProcessExists(pid, Name) {
		return fmt.Errorf("process %v/%v exists", Name, pid)
	}
	if err := fork(); err != nil {
		return fmt.Errorf("failed to fork: %v", err)
	}
	return nil
}

func kill(pid int) (err error) {
	err = syscall.Kill(pid, syscall.SIGTERM)
	if err != nil {
		return
	}
	const interval = 500 * time.Millisecond
	maxCnt := int(10 * time.Second / interval)
	for cnt := 0; cnt < maxCnt; cnt++ {
		time.Sleep(interval)
		if !ProcessExists(pid, Name) {
			return nil
		}
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func stop() (err error) {
	pid, err := readPID()
	if err != nil {
		return
	}
	err = kill(pid)
	if err != nil {
		return
	}
	return syscall.Unlink(path.Join("/run/", Name+".pid"))
}

func reload() (err error) {
	pid, err := readPID()
	if err != nil {
		return
	}
	if !ProcessExists(pid, Name) {
		return fmt.Errorf("process %v/%v not exists", Name, pid)
	}
	// send sighup signal to notify to reload
	return syscall.Kill(pid, syscall.SIGHUP)
}

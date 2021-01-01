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
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	if godaemon.Stage() == godaemon.StageDaemon {
		// init daemon
		err := daemonInit()
		if err != nil {
			log.Fatalln(err)
		}
		// init config
		_ = GetConfig()
	}
}

func start() error {
	if pid, err := readPIDFile(); err != nil && !os.IsNotExist(err) {
		return err
	} else if processExists(pid, Name) {
		return fmt.Errorf("process %v/%v exists", Name, pid)
	}
	_ = writePIDFile()
	if err := fork(); err != nil {
		return fmt.Errorf("failed to fork: %v", err)
	}
	return nil
}

func stop() (err error) {
	pid, err := readPIDFile()
	if err != nil {
		return
	}
	err = kill(pid)
	if err != nil {
		return
	}
	_ = syscall.Unlink(path.Join("/run/", Name+".pid"))
	return nil
}

func reload() (err error) {
	pid, err := readPIDFile()
	if err != nil {
		return
	}
	if !processExists(pid, Name) {
		return fmt.Errorf("process %v/%v not exists", Name, pid)
	}
	// send sighup signal to notify to reload
	return syscall.Kill(pid, syscall.SIGHUP)
}

func daemonInit() (err error) {
	DaemonMode = true

	// parse and redirect output
	GetConfig()

	err = writePIDFile()
	if err != nil {
		return
	}
	return
}

// https://github.com/golang/go/issues/15538
// We can not safely syscall.Fork in a multi-threaded program.
func fork() (err error) {
	_, _, err = godaemon.MakeDaemon(&godaemon.DaemonAttr{})
	return
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
		if !processExists(pid, Name) {
			return nil
		}
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func readPIDFile() (pid int, err error) {
	b, err := ioutil.ReadFile(path.Join("/run/", Name+".pid"))
	if err != nil {
		return
	}
	return strconv.Atoi(strings.TrimSpace(string(b)))
}

// pass pname="" if not check process name
func processExists(pid int, pname string) bool {
	pn, err := getProcessName(pid)
	if err != nil {
		return false
	}
	if pname == "" {
		return true
	}
	return pn == pname
}

func getProcessName(pid int) (string, error) {
	f, err := os.Open(path.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	var line []byte
	for err == nil {
		line, _, err = r.ReadLine()
		fields := bytes.Fields(line)
		if len(fields) < 2 || bytes.EqualFold(fields[0], []byte("Name")) {
			continue
		}
		return string(fields[1]), nil
	}
	return "", nil
}

func writePIDFile() (err error) {
	err = ioutil.WriteFile(path.Join("/run/", Name+".pid"), []byte(strconv.Itoa(os.Getpid())), 0644)
	if err != nil {
		return fmt.Errorf("writePIDFile: %v", err)
	}
	return nil
}

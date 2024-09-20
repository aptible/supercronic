package main

import (
	"os"
	"syscall"

	reaper "github.com/ramr/go-reaper"
	"github.com/sirupsen/logrus"
)

func forkExec() {
	//  Start background reaping of orphaned child processes.
	go reaper.Reap()

	pwd, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Failed to get current working directory: %s", err)
		return
	}

	var wstatus syscall.WaitStatus
	pattrs := &syscall.ProcAttr{
		Dir: pwd,
		Env: os.Environ(),
		Sys: &syscall.SysProcAttr{Setsid: true},
		Files: []uintptr{
			uintptr(syscall.Stdin),
			uintptr(syscall.Stdout),
			uintptr(syscall.Stderr),
		},
	}
	pid, err := syscall.ForkExec(os.Args[0], os.Args, pattrs)
	if err != nil {
		logrus.Fatalf("Failed to fork exec: %s", err)
		return
	}

	_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	for syscall.EINTR == err {
		_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	}
	if err != nil {
		logrus.Fatalf("Failed to wait: %s", err)
		return
	}
	os.Exit(wstatus.ExitStatus())
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	reaper "github.com/ramr/go-reaper"
	"github.com/sirupsen/logrus"
)

func forkExec() {
	//  Start background reaping of orphaned child processes.
	go reaper.Reap()

	pwd, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Failed to get current working directory: %s", err.Error())
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
		logrus.Fatalf("Failed to fork exec: %s", err.Error())
		return
	}

	signalToFork(pid)

	_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	for syscall.EINTR == err {
		fmt.Println("wait4 got ", err.Error())
		_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	}

	if err != nil {
		// FIXME: may be reaped by reaper.....
		// if err != syscall.ECHILD {
		logrus.Errorf("Failed to wait: %s", err.Error())
		// }
		return
	}
	os.Exit(wstatus.ExitStatus())
}

func signalToFork(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		logrus.Fatalf("Failed findProcess pid:%d,%s", pid, err.Error())
	}
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGUSR2)
	go func() {
		s := <-termChan
		logrus.Infof("Got signal: %v,%s", s, s.String())
		if err := p.Signal(s); err != nil {
			logrus.Errorf("Failed to send signal to child: %s", err.Error())
		}
	}()
}

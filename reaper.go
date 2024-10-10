package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

func forkExec() {

	// run supercronic in other process
	pwd, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Failed to get current working directory: %s", err.Error())
		return
	}

	pattrs := &syscall.ProcAttr{
		Dir: pwd,
		Env: os.Environ(),
		Files: []uintptr{
			uintptr(syscall.Stdin),
			uintptr(syscall.Stdout),
			uintptr(syscall.Stderr),
		},
	}
	args := make([]string, 0, len(os.Args)+1)
	// disable reaping for supercronic, avoid no sense warning
	args = append(args, os.Args[0], "-no-reap")
	args = append(args, os.Args[1:]...)

	pid, err := syscall.ForkExec(args[0], args, pattrs)
	if err != nil {
		logrus.Fatalf("Failed to fork exec: %s", err.Error())
		return
	}

	// forward signal to supercronic
	signalToFork(pid)
	// got supercronic exit status
	wstatus := reapChildren(pid)
	os.Exit(wstatus.ExitStatus())
}

func signalToFork(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		logrus.Fatalf("Failed findProcess supercronic pid:%d,%s", pid, err.Error())
	}
	termChan := make(chan os.Signal, 1)
	signal.Notify(termChan, signalList...)
	go func() {
		for {
			s := <-termChan
			if err := p.Signal(s); err != nil {
				logrus.Errorf("Failed to send signal to supercronic: %s", err.Error())
			}
		}
	}()
}

// copy from https://github.com/ramr/go-reaper
// modify for wait exit status of supercronic
// without modify, supercronic exit status may not be obtained

// Be a good parent - clean up behind the children.
func reapChildren(superCrondPid int) syscall.WaitStatus {
	var notifications = make(chan os.Signal, 1)

	go sigChildHandler(notifications)

	// all child
	const rpid = -1
	var wstatus syscall.WaitStatus

	for {
		var sig = <-notifications
		logrus.Debugf("reaper received signal %v\n", sig)
		for {
			pid, err := syscall.Wait4(rpid, &wstatus, 0, nil)
			for syscall.EINTR == err {
				pid, err = syscall.Wait4(pid, &wstatus, 0, nil)
			}

			if syscall.ECHILD == err {
				break
			}

			if superCrondPid == pid {
				logrus.Debugf("supercronic exit, pid=%d, wstatus=%+v, err=%+v\n", pid, wstatus, err)
				return wstatus
			}
			// note: change output need change test
			logrus.Warnf("reaper cleanup: pid=%d, wstatus=%+v\n",
				pid, wstatus)
		}
	}

}

// Handle death of child (SIGCHLD) messages. Pushes the signal onto the
// notifications channel if there is a waiter.
func sigChildHandler(notifications chan os.Signal) {
	var sigs = make(chan os.Signal, 3)
	signal.Notify(sigs, syscall.SIGCHLD)

	for {
		var sig = <-sigs
		select {
		case notifications <- sig: /*  published it.  */
		default:
			/*
			 *  Notifications channel full - drop it to the
			 *  floor. This ensures we don't fill up the SIGCHLD
			 *  queue. The reaper just waits for any child
			 *  process (pid=-1), so we ain't loosing it!! ;^)
			 */
		}
	}

} /*  End of function  sigChildHandler.  */

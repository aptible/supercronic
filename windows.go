//go:build windows

package main

import (
	"os"
	"syscall"

	"github.com/sirupsen/logrus"
)

// forkExec is a no-op on Windows; process reaping is a Linux-only concept
// and supercronic will never run as PID 1 on Windows.
func forkExec() {
	logrus.Fatal("process reaping is not supported on Windows")
}

// reloadSig is a synthetic sentinel used to trigger crontab reloads on
// platforms that do not support SIGUSR2 (e.g. via fsnotify file watch).
type reloadSig struct{}

func (reloadSig) String() string { return "reload" }
func (reloadSig) Signal()        {}

// reloadSignal is the signal used to trigger a live crontab reload.
var reloadSignal os.Signal = reloadSig{}

// signalList contains the OS signals supercronic listens for on Windows.
// reloadSignal is intentionally excluded as it is a synthetic sentinel that
// cannot be delivered by the OS; reloads are triggered via fsnotify instead.
var signalList = []os.Signal{
	syscall.SIGINT, syscall.SIGTERM,
}

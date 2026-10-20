//go:build !windows

package cron

import "syscall"

// sysProcAttr returns a SysProcAttr that runs the child in its own process
// group so that CTRL+C in interactive usage stops supercronic, not the child.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

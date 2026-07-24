//go:build windows

package cron

import "syscall"

// sysProcAttr returns an empty SysProcAttr; Setpgid is not supported on Windows.
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{}
}

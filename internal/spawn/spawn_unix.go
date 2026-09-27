//go:build unix

package spawn

import "syscall"

func detachedAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

//go:build darwin

package terminalsetup

import (
	"os"
	"syscall"
)

type syscallStat = syscall.Stat_t

func inodeOf(fi os.FileInfo) uint64 {
	if s, ok := fi.Sys().(*syscallStat); ok {
		return s.Ino
	}
	return 0
}

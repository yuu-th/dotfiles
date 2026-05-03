//go:build !darwin

package terminalsetup

import "os"

type syscallStat struct{}

func inodeOf(_ os.FileInfo) uint64 { return 0 }

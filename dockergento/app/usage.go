package app

import (
	"os"
	"syscall"
)

// usage is what a file takes up on disk, which is what `du` reports and what somebody deciding
// whether to delete a copy is actually looking at.
//
// The length of the file is the other answer, and it is the wrong one here: a dump takes the
// blocks it takes, and on the filesystems this runs on that is never fewer than one.
func usage(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return stat.Blocks * 512
	}

	return info.Size()
}

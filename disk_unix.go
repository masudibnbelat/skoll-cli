//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

// freeDiskSpace reports the free bytes on the disk holding path,
// and whether the number could be read. Unix only (syscall.Statfs);
// Windows uses disk_windows.go. The path itself may not exist yet
// (a download target), so the nearest existing parent is used.
func freeDiskSpace(path string) (uint64, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	for {
		if _, err := os.Stat(abs); err == nil {
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return 0, false
		}
		abs = parent
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(abs, &st); err != nil {
		return 0, false
	}
	return st.Bavail * uint64(st.Bsize), true
}

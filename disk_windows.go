//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetDiskFreeSpaceEx = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// freeDiskSpace reports the free bytes on the disk holding path,
// via GetDiskFreeSpaceExW (no extra dependency — kernel32 is part
// of Windows). The path itself may not exist yet (a download
// target), so the nearest existing parent is used, like the Unix
// version.
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
	// A drive root needs its trailing backslash ("C:\").
	dir := abs
	if len(dir) == 2 && dir[1] == ':' {
		dir += "\\"
	}
	pathPtr, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return 0, false
	}
	var freeBytesAvailable uint64
	ret, _, _ := procGetDiskFreeSpaceEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		0,
		0,
	)
	if ret == 0 {
		return 0, false
	}
	return freeBytesAvailable, true
}

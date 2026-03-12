//go:build windows

package engine

import (
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"
)

// cleanupMode waits for the install directory to be unlocked, then removes it.
// Invoked from a temp copy of the binary after the uninstaller exits.
func cleanupMode(installDir string) {
	// Wait up to 30s for the original process to release the install dir.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		probe := installDir + "._probe"
		if err := os.Rename(installDir, probe); err == nil {
			os.Rename(probe, installDir)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Try to remove the install directory up to 5 times.
	for i := 0; i < 5; i++ {
		if err := os.RemoveAll(installDir); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Schedule self-deletion on next reboot.
	if exePath, err := os.Executable(); err == nil {
		scheduleDeleteOnReboot(exePath)
	}
}

// scheduleDeleteOnReboot uses MoveFileExW with MOVEFILE_DELAY_UNTIL_REBOOT
// to delete the file on the next system reboot.
func scheduleDeleteOnReboot(path string) {
	const movefileDelayUntilReboot = 4
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	moveFileEx := kernel32.NewProc("MoveFileExW")
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	moveFileEx.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0, // null destination = delete
		movefileDelayUntilReboot,
	)
}

// launchDetached starts an executable in a new detached process group.
func launchDetached(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	return cmd.Start()
}

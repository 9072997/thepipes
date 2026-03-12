//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	desktopPath  = `C:\Users\Public\Desktop`
	startMenuPath = `C:\ProgramData\Microsoft\Windows\Start Menu\Programs`
)

// CreateAll creates Desktop and Start Menu shortcuts from the shortcuts map.
// vars is used to expand %variable% tokens in shortcut targets.
func CreateAll(shortcuts map[string]string, vars map[string]string) error {
	// COM is per-OS-thread; pin this goroutine so CoInitializeEx
	// remains valid for all subsequent COM calls.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("CoInitialize: %w", err)
	}
	defer ole.CoUninitialize()

	for name, target := range shortcuts {
		// Expand variables
		for k, v := range vars {
			target = strings.ReplaceAll(target, "%"+k+"%", v)
		}

		isURL := strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")

		// Resolve relative paths against the install directory
		if !isURL && !filepath.IsAbs(target) {
			target = filepath.Join(vars["install"], target)
		}

		isDir := false
		if !isURL {
			if info, err := os.Stat(target); err == nil && info.IsDir() {
				isDir = true
			}
		}

		for _, dir := range []string{desktopPath, startMenuPath} {
			if isURL {
				if err := createURLShortcut(dir, name, target); err != nil {
					return fmt.Errorf("create URL shortcut %q in %s: %w", name, dir, err)
				}
			} else if isDir {
				if err := createFolderShortcut(dir, name, target); err != nil {
					return fmt.Errorf("create folder shortcut %q in %s: %w", name, dir, err)
				}
			} else {
				if err := createLNKShortcut(dir, name, target); err != nil {
					return fmt.Errorf("create LNK shortcut %q in %s: %w", name, dir, err)
				}
			}
		}
	}
	return nil
}

// RemoveAll deletes shortcuts for the given names from both Desktop and Start Menu.
func RemoveAll(shortcuts map[string]string) {
	for name := range shortcuts {
		for _, dir := range []string{desktopPath, startMenuPath} {
			os.Remove(dir + `\` + name + ".lnk")
			os.Remove(dir + `\` + name + ".url")
			os.RemoveAll(dir + `\` + name) // folder shortcut
		}
	}
}

func createURLShortcut(dir, name, url string) error {
	path := dir + `\` + name + ".url"
	content := "[InternetShortcut]\r\nURL=" + url + "\r\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func createLNKShortcut(dir, name, target string) error {
	wshell, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return fmt.Errorf("create WScript.Shell: %w", err)
	}
	defer wshell.Release()

	dispatch, err := wshell.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("QueryInterface: %w", err)
	}
	defer dispatch.Release()

	lnkPath := dir + `\` + name + ".lnk"
	cs, err := oleutil.CallMethod(dispatch, "CreateShortcut", lnkPath)
	if err != nil {
		return fmt.Errorf("CreateShortcut: %w", err)
	}
	shortcut := cs.ToIDispatch()
	defer shortcut.Release()

	if _, err := oleutil.PutProperty(shortcut, "TargetPath", target); err != nil {
		return fmt.Errorf("set TargetPath: %w", err)
	}
	if _, err := oleutil.CallMethod(shortcut, "Save"); err != nil {
		return fmt.Errorf("save shortcut: %w", err)
	}
	return nil
}

// createFolderShortcut creates a Windows folder shortcut — a directory
// containing desktop.ini and target.lnk that Explorer treats as a
// transparent link to the target folder (browsable in-place).
func createFolderShortcut(dir, name, target string) error {
	shortcutDir := filepath.Join(dir, name)
	if err := os.MkdirAll(shortcutDir, 0755); err != nil {
		return fmt.Errorf("create shortcut dir: %w", err)
	}

	// desktop.ini marks this directory as a folder shortcut
	desktopIni := filepath.Join(shortcutDir, "desktop.ini")
	iniContent := "[.ShellClassInfo]\r\nCLSID2={0AFACED1-E828-11D1-9187-B532F1E9575D}\r\nFlags=2\r\nConfirmFileOp=0\r\n"
	if err := os.WriteFile(desktopIni, []byte(iniContent), 0644); err != nil {
		return fmt.Errorf("write desktop.ini: %w", err)
	}

	// target.lnk points to the actual folder
	if err := createLNKShortcut(shortcutDir, "target", target); err != nil {
		return fmt.Errorf("create target.lnk: %w", err)
	}

	// Mark desktop.ini and the shortcut directory as System+Hidden
	setHiddenSystem(desktopIni)
	setReadOnly(shortcutDir)

	return nil
}

func setHiddenSystem(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_HIDDEN|syscall.FILE_ATTRIBUTE_SYSTEM)
}

func setReadOnly(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	syscall.SetFileAttributes(p, syscall.FILE_ATTRIBUTE_READONLY)
}

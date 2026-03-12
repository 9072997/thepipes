//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

		for _, dir := range []string{desktopPath, startMenuPath} {
			if isURL {
				if err := createURLShortcut(dir, name, target); err != nil {
					return fmt.Errorf("create URL shortcut %q in %s: %w", name, dir, err)
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

	// For directory targets, use explorer.exe to open the folder
	targetPath := target
	arguments := ""
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		targetPath = filepath.Join(os.Getenv("SystemRoot"), "explorer.exe")
		arguments = target
	}

	if _, err := oleutil.PutProperty(shortcut, "TargetPath", targetPath); err != nil {
		return fmt.Errorf("set TargetPath: %w", err)
	}
	if arguments != "" {
		if _, err := oleutil.PutProperty(shortcut, "Arguments", arguments); err != nil {
			return fmt.Errorf("set Arguments: %w", err)
		}
	}
	if _, err := oleutil.CallMethod(shortcut, "Save"); err != nil {
		return fmt.Errorf("save shortcut: %w", err)
	}
	return nil
}

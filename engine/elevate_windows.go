//go:build windows

package engine

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// isAdmin reports whether the current process is running with
// administrator privileges (i.e. the process token is a member of
// the built-in Administrators group S-1-5-32-544).
func isAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.GetCurrentProcessToken()
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}

var (
	modShell32        = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteW = modShell32.NewProc("ShellExecuteW")
)

// selfElevate re-launches the current process elevated via the
// "runas" verb (triggers a UAC prompt). On success it exits the
// current (unelevated) process; on failure (user cancelled UAC)
// it exits with code 1.
func selfElevate() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}

	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString(strings.Join(os.Args[1:], " "))

	ret, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(args)),
		0,
		windows.SW_NORMAL,
	)

	// ShellExecuteW returns >32 on success.
	if ret > 32 {
		os.Exit(0)
	}
	os.Exit(1)
}

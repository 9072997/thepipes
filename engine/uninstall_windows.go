//go:build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/internal/eventlog"
	"github.com/9072997/thepipes/internal/firewall"
	"github.com/9072997/thepipes/internal/regutil"
	"github.com/9072997/thepipes/internal/shortcut"
	"github.com/9072997/thepipes/internal/svcmgr"
)

func uninstallMode(manifest AppManifest) {
	installDir := getInstallDir(manifest.Name)
	dataDir := getDataDir(manifest.Name)
	svcName := slug(manifest.Name)

	// Show confirmation dialog via Tk.
	tk.App.WmTitle("Uninstall " + manifest.Name)
	tk.App.SetResizable(false, false)
	tk.WmGeometry(tk.App, "480x200")

	mainF := tk.App.Frame()
	tk.Pack(mainF, tk.Fill("both"), tk.Expand(true), tk.Padx("12"), tk.Pady("12"))

	msg := fmt.Sprintf("Are you sure you want to uninstall %s?\n\nThis will remove the application and all associated files.", manifest.Name)
	lbl := mainF.Label(tk.Txt(msg), tk.Wraplength(440), tk.Justify("left"), tk.Anchor("w"))
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"), tk.Pady("8"))

	btnF := mainF.Frame()
	tk.Pack(btnF, tk.Side("bottom"), tk.Fill("x"))

	confirmed := false
	tk.Pack(
		btnF.TButton(tk.Txt("Uninstall"), tk.Command(func() {
			confirmed = true
			tk.Destroy(tk.App)
		})),
		tk.Side("right"), tk.Padx("4"),
	)
	tk.Pack(
		btnF.TButton(tk.Txt("Cancel"), tk.Command(func() {
			tk.Destroy(tk.App)
		})),
		tk.Side("right"),
	)

	tk.App.Wait()

	if !confirmed {
		return
	}

	// Perform uninstall steps.
	var errs []string
	addErr := func(msg string) { errs = append(errs, msg) }

	// 1. Stop the service.
	if err := svcmgr.Stop(svcName); err != nil {
		addErr(fmt.Sprintf("stop service: %v", err))
	}
	// 2. Delete the service.
	if err := svcmgr.Delete(svcName); err != nil {
		addErr(fmt.Sprintf("delete service: %v", err))
	}
	// 3. Remove firewall rule.
	_ = firewall.Remove(manifest.Name)

	// 4. Remove shortcuts.
	shortcut.RemoveAll(manifest.Shortcuts)

	// 5. Remove ARP entry.
	_ = regutil.RemoveARP(manifest.Name)

	// 6. Unregister Event Log source.
	_ = eventlog.Unregister(manifest.Name)

	// 7. Remove update state.
	_ = regutil.RemoveUpdateState(manifest.Name)

	// 8. Ask about data directory.
	keepData := askKeepData(dataDir)
	if !keepData {
		// 9. Delete data directory.
		_ = os.RemoveAll(dataDir)
	}

	// 10. Copy self to temp and launch cleanup.
	tempExe, err := copyToTemp()
	if err != nil {
		// Can't do deferred cleanup; just try to remove install dir now.
		_ = os.RemoveAll(installDir)
		os.Exit(0)
		return
	}

	// 11. Launch temp copy as cleanup, then exit.
	cmd := exec.Command(tempExe, "--cleanup", installDir)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	_ = cmd.Start()
	os.Exit(0)
}

// askKeepData shows a dialog asking whether to keep the data directory.
// Returns true if the user wants to keep it.
func askKeepData(dataDir string) bool {
	keep := true

	// Re-init Tk for the second dialog.
	tk.App.WmTitle("Keep Data?")
	tk.App.SetResizable(false, false)
	tk.WmGeometry(tk.App, "480x180")

	mainF := tk.App.Frame()
	tk.Pack(mainF, tk.Fill("both"), tk.Expand(true), tk.Padx("12"), tk.Pady("12"))

	msg := fmt.Sprintf("Keep configuration and data in:\n%s", dataDir)
	lbl := mainF.Label(tk.Txt(msg), tk.Wraplength(440), tk.Justify("left"), tk.Anchor("w"))
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"), tk.Pady("8"))

	btnF := mainF.Frame()
	tk.Pack(btnF, tk.Side("bottom"), tk.Fill("x"))

	tk.Pack(
		btnF.TButton(tk.Txt("Keep"), tk.Command(func() {
			keep = true
			tk.Destroy(tk.App)
		})),
		tk.Side("right"), tk.Padx("4"),
	)
	tk.Pack(
		btnF.TButton(tk.Txt("Delete"), tk.Command(func() {
			keep = false
			tk.Destroy(tk.App)
		})),
		tk.Side("right"),
	)

	tk.App.Wait()
	return keep
}

// copyToTemp copies the running executable to a temp file and returns its path.
func copyToTemp() (string, error) {
	tmpPath := filepath.Join(os.TempDir(), slug("")+"-cleanup.exe")
	return tmpPath, copySelf(tmpPath)
}

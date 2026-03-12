//go:build windows

package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/9072997/thepipes/internal/eventlog"
	"github.com/9072997/thepipes/internal/firewall"
	"github.com/9072997/thepipes/internal/installcfg"
	"github.com/9072997/thepipes/internal/regutil"
	"github.com/9072997/thepipes/internal/shortcut"
	"github.com/9072997/thepipes/internal/svcmgr"
)

// Install runs the 9-step installation sequence.
// Status lines are written to w as each step completes.
func Install(manifest AppManifest, vars map[string]string, w io.Writer) error {
	installDir := getInstallDir(manifest.Name)
	dataDir := getDataDir(manifest.Name)
	svcName := slug(manifest.Name)
	binPath := installBinaryPath(manifest)

	send := func(msg string) {
		if w != nil {
			fmt.Fprintln(w, msg)
		}
	}

	// Step 1: Extract embedded payload to install directory.
	send("Extracting files...")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("create install dir: %w", err)
	}
	if err := extractPayload(manifest.PayloadFS, installDir); err != nil {
		return fmt.Errorf("extract payload: %w", err)
	}

	// Copy self to install directory as the persistent service manager binary.
	send("Copying installer binary...")
	if err := copySelf(binPath); err != nil {
		return fmt.Errorf("copy installer binary: %w", err)
	}

	// Step 2: Create data directory.
	send("Creating data directory...")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Step 3: Write install-config.json.
	send("Writing configuration...")
	if err := installcfg.Write(dataDir, vars); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Build merged vars for shortcut expansion.
	allVars := MergeVars(BuiltinVars(manifest), vars)

	// Step 4: Register Windows service.
	send("Registering Windows service...")
	svcAccount := vars["service_account"]
	svcPassword := vars[".service_password"]
	if err := svcmgr.Install(
		svcName,
		manifest.Name,
		manifest.Name+" service",
		binPath,
		svcAccount,
		svcPassword,
	); err != nil {
		return fmt.Errorf("register service: %w", err)
	}

	// Step 5: Add Windows Firewall rule.
	send("Adding firewall rule...")
	exePath := filepath.Join(installDir, firstToken(Expand(manifest.Command, allVars)))
	if err := firewall.Add(manifest.Name, exePath); err != nil {
		// Non-fatal - log but continue.
		send(fmt.Sprintf("Warning: firewall rule: %v", err))
	}

	// Step 6: Create shortcuts.
	send("Creating shortcuts...")
	if len(manifest.Shortcuts) > 0 {
		if err := shortcut.CreateAll(manifest.Shortcuts, allVars); err != nil {
			send(fmt.Sprintf("Warning: shortcuts: %v", err))
		}
	}

	// Step 7: Register Event Log source.
	send("Registering Event Log source...")
	if err := eventlog.Register(manifest.Name); err != nil {
		send(fmt.Sprintf("Warning: eventlog register: %v", err))
	}

	// Step 8: Write Add/Remove Programs entry.
	send("Registering in Add/Remove Programs...")
	uninstallCmd := fmt.Sprintf(`"%s" --uninstall`, binPath)
	if err := regutil.RegisterARP(
		manifest.Name,
		manifest.Version,
		manifest.Publisher,
		installDir,
		uninstallCmd,
		0,
	); err != nil {
		return fmt.Errorf("register ARP: %w", err)
	}

	// Step 9: Write update state and start service.
	send("Initialising update state...")
	if manifest.UpdateURL != "" {
		hash, _ := hashFile(binPath)
		_ = regutil.WriteUpdateState(manifest.Name, hash, "", "")
	}

	send("Starting service...")
	if err := svcmgr.Start(svcName); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	send("Installation complete.")
	return nil
}

// extractPayload writes the embedded FS to destDir.
// Prefix stripping is handled by cleanfs, so we walk from "." directly.
func extractPayload(fsys fs.FS, destDir string) error {
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}

		dest := filepath.Join(destDir, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0644)
	})
}

// copySelf copies the running executable to destPath.
func copySelf(destPath string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	src, err := os.Open(exePath)
	if err != nil {
		return err
	}
	defer src.Close()

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// hashFile computes the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

//go:build windows

package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/9072997/thepipes/internal/regutil"
	"github.com/9072997/thepipes/internal/svcmgr"
)

// updateMode applies a downloaded update.
// The new binary (self) replaces the installed binary and payload, then restarts the service.
func updateMode(manifest AppManifest, installDir string) error {
	installDir = getInstallDir(manifest.Name)
	svcName := slug(manifest.Name)

	// Wait up to 30s for the service to stop.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		state, _ := svcmgr.State(svcName)
		if state == 1 { // svc.Stopped == 1
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	_ = svcmgr.Stop(svcName)

	// Extract own PayloadFS to installDir (skips install-config.json in dataDir).
	if err := extractPayload(manifest.PayloadFS, installDir); err != nil {
		return fmt.Errorf("extract payload: %w", err)
	}

	// Copy self to the service manager binary path.
	binPath := filepath.Join(installDir, binaryName(manifest.Name))
	if err := copySelf(binPath); err != nil {
		return fmt.Errorf("copy self: %w", err)
	}

	// Update registry state.
	hash, _ := hashFile(binPath)
	exePath, _ := os.Executable()
	etag := ""
	lastMod := ""
	curHash, curETag, curLastMod, _ := regutil.ReadUpdateState(manifest.Name)
	_ = curHash
	if curETag != "" {
		etag = curETag
	}
	if curLastMod != "" {
		lastMod = curLastMod
	}
	_ = regutil.WriteUpdateState(manifest.Name, hash, etag, lastMod)

	// Restart service.
	if err := svcmgr.Start(svcName); err != nil {
		return fmt.Errorf("start service: %w", err)
	}

	// Schedule self-deletion on reboot.
	scheduleDeleteOnReboot(exePath)
	return nil
}

// applyUpdateMode extracts the payload to installDir without any service operations.
func applyUpdateMode(manifest AppManifest, installDir string) error {
	return extractPayload(manifest.PayloadFS, installDir)
}

// checkForUpdateLoop runs an update check at startup, then every hour.
func checkForUpdateLoop(manifest AppManifest, installDir string, logW io.Writer) {
	log := func(msg string) {
		if logW != nil {
			fmt.Fprintf(logW, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
		}
	}

	for {
		if err := checkForUpdate(manifest, installDir); err != nil {
			log(fmt.Sprintf("Update check error: %v", err))
		}
		time.Sleep(1 * time.Hour)
	}
}

// checkForUpdate performs one update check cycle.
func checkForUpdate(manifest AppManifest, installDir string) error {
	if manifest.UpdateURL == "" {
		return nil
	}

	binPath := filepath.Join(installDir, binaryName(manifest.Name))
	currentHash, err := hashFile(binPath)
	if err != nil {
		return fmt.Errorf("hash binary: %w", err)
	}

	storedHash, storedETag, storedLastMod, _ := regutil.ReadUpdateState(manifest.Name)

	// Make a HEAD request with conditional headers.
	req, err := http.NewRequest("HEAD", manifest.UpdateURL, nil)
	if err != nil {
		return fmt.Errorf("build HEAD request: %w", err)
	}
	if storedETag != "" {
		req.Header.Set("If-None-Match", storedETag)
	}
	if storedLastMod != "" {
		req.Header.Set("If-Modified-Since", storedLastMod)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HEAD %s: %w", manifest.UpdateURL, err)
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if storedHash != "" && storedHash != currentHash {
			// Binary changed externally - trigger update.
			return triggerUpdate(manifest, installDir, storedETag, storedLastMod)
		}
		return nil

	case http.StatusOK:
		newETag := resp.Header.Get("ETag")
		newLastMod := resp.Header.Get("Last-Modified")

		// Download to temp file.
		tmpPath, newHash, err := downloadToTemp(manifest.Name, manifest.UpdateURL)
		if err != nil {
			return fmt.Errorf("download update: %w", err)
		}

		if newHash == currentHash {
			// Same content; just refresh headers.
			_ = regutil.WriteUpdateState(manifest.Name, currentHash, newETag, newLastMod)
			os.Remove(tmpPath)
			return nil
		}

		// Verify PE header (MZ magic bytes).
		if !isPEFile(tmpPath) {
			os.Remove(tmpPath)
			return fmt.Errorf("downloaded file is not a valid PE executable")
		}

		// Store new headers; launch update.
		_ = regutil.WriteUpdateState(manifest.Name, storedHash, newETag, newLastMod)
		return launchDetached(tmpPath, "--update", installDir)

	default:
		return fmt.Errorf("unexpected HEAD status: %d", resp.StatusCode)
	}
}

func triggerUpdate(manifest AppManifest, installDir, etag, lastMod string) error {
	tmpPath, _, err := downloadToTemp(manifest.Name, manifest.UpdateURL)
	if err != nil {
		return err
	}
	if !isPEFile(tmpPath) {
		os.Remove(tmpPath)
		return fmt.Errorf("downloaded file is not a valid PE executable")
	}
	return launchDetached(tmpPath, "--update", installDir)
}

func downloadToTemp(appName, url string) (path, hash string, err error) {
	tmpPath := filepath.Join(os.TempDir(), appName+"-update.exe")
	resp, err := http.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	f, err := os.Create(tmpPath)
	if err != nil {
		return "", "", fmt.Errorf("create temp file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return "", "", fmt.Errorf("write temp: %w", err)
	}
	return tmpPath, hex.EncodeToString(h.Sum(nil)), nil
}

func isPEFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 2)
	if _, err := io.ReadFull(f, magic); err != nil {
		return false
	}
	return magic[0] == 'M' && magic[1] == 'Z'
}

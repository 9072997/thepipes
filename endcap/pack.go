//go:build windows

package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/9072997/thepipes/engine"
)

// pack validates the payload directory, creates a ZIP, and writes a new EXE
// with the ZIP appended to the current working directory.
func pack(dir string) error {
	// Refuse if this EXE already has a ZIP appended.
	if _, err := readSelfZip(); err == nil {
		return fmt.Errorf("this executable already has a payload; use a bare carrier")
	}

	// Read and validate AppManifest.json.
	manifestData, err := os.ReadFile(filepath.Join(dir, "AppManifest.json"))
	if err != nil {
		return fmt.Errorf("reading AppManifest.json: %w", err)
	}
	var manifest engine.AppManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parsing AppManifest.json: %w", err)
	}
	if manifest.Name == "" {
		return fmt.Errorf("AppManifest.json: Name is required")
	}
	if manifest.Command == "" {
		return fmt.Errorf("AppManifest.json: Command is required")
	}

	// Read and validate WizardPages.json.
	pagesData, err := os.ReadFile(filepath.Join(dir, "WizardPages.json"))
	if err != nil {
		return fmt.Errorf("reading WizardPages.json: %w", err)
	}
	if err := validatePages(pagesData, dir); err != nil {
		return err
	}

	// Create ZIP from the directory.
	var zipBuf bytes.Buffer
	if err := createZip(&zipBuf, dir); err != nil {
		return fmt.Errorf("creating ZIP: %w", err)
	}

	// Read the carrier EXE (ourselves).
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exeData, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("reading carrier: %w", err)
	}

	// Write output: carrier + ZIP.
	outName := slug(manifest.Name)
	if manifest.Version != "" {
		outName += "-" + manifest.Version
	}
	outName += "-setup.exe"
	out, err := os.Create(outName)
	if err != nil {
		return fmt.Errorf("creating %s: %w", outName, err)
	}
	defer out.Close()

	if _, err := out.Write(exeData); err != nil {
		return err
	}
	if _, err := zipBuf.WriteTo(out); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "Created %s\n", outName)
	return nil
}

// validatePages checks that WizardPages.json is well-formed and contains an
// Install page. dir is the payload directory, used to stat referenced scripts.
func validatePages(data []byte, dir string) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("WizardPages.json: %w", err)
	}

	hasInstall := false
	for i, elem := range raw {
		var spec pageSpec
		if len(elem) > 0 && elem[0] == '"' {
			var typeName string
			if err := json.Unmarshal(elem, &typeName); err != nil {
				return fmt.Errorf("WizardPages.json page %d: %w", i, err)
			}
			spec = pageSpec{Type: typeName}
		} else {
			if err := json.Unmarshal(elem, &spec); err != nil {
				return fmt.Errorf("WizardPages.json page %d: %w", i, err)
			}
		}

		switch spec.Type {
		case "Welcome", "Simple", "Process", "InstallLocation", "ServiceAccount",
			"AutoUpdates", "Review", "Install", "Complete", "License":
			// valid
		default:
			return fmt.Errorf("WizardPages.json page %d: unknown type %q", i, spec.Type)
		}

		if spec.Type == "Process" {
			if spec.Func == "" {
				return fmt.Errorf("WizardPages.json page %d: Process requires Func", i)
			}
			if _, err := os.Stat(filepath.Join(dir, spec.Func)); err != nil {
				return fmt.Errorf("WizardPages.json page %d: Func %q: %w", i, spec.Func, err)
			}
		}

		if spec.Type == "Simple" && spec.ValidateFunc != "" {
			if _, err := os.Stat(filepath.Join(dir, spec.ValidateFunc)); err != nil {
				return fmt.Errorf("WizardPages.json page %d: ValidateFunc %q: %w", i, spec.ValidateFunc, err)
			}
		}

		if spec.Type == "License" {
			if spec.File == "" {
				return fmt.Errorf("WizardPages.json page %d: License requires File", i)
			}
			if _, err := os.Stat(filepath.Join(dir, spec.File)); err != nil {
				return fmt.Errorf("WizardPages.json page %d: File %q: %w", i, spec.File, err)
			}
		}

		if spec.Type == "Install" {
			hasInstall = true
		}
	}

	if !hasInstall {
		return fmt.Errorf("WizardPages.json must contain an Install page")
	}
	return nil
}

// createZip writes a ZIP archive of dir's contents to w.
func createZip(w io.Writer, dir string) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		// ZIP uses forward slashes.
		rel = filepath.ToSlash(rel)

		if d.IsDir() {
			_, err := zw.Create(rel + "/")
			return err
		}

		fw, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(fw, f)
		return err
	})
}

// slug converts a display name to a filename-safe identifier.
// "My Cool App" -> "my-cool-app"
func slug(name string) string {
	var sb strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			prev = r
		} else if prev != '-' {
			sb.WriteRune('-')
			prev = '-'
		}
	}
	s := strings.TrimSuffix(sb.String(), "-")
	if s == "" {
		s = "app"
	}
	return s
}

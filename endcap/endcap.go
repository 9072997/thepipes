//go:build windows

package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"

	"github.com/9072997/thepipes/engine"
	"github.com/9072997/thepipes/internal/cleanfs"
	"github.com/9072997/thepipes/pages"
)

// pageSpec describes a single wizard page from WizardPages.json.
type pageSpec struct {
	Type         string
	PageTitle    string        // Simple, Process
	Fields       []pages.Field // Simple only
	Func         string        // Process only - tengo script path in ZIP
	Reentry      bool          // Process only
	ValidateFunc string        // Simple only - tengo script path in ZIP
	File         string        // License only - path to license text file in ZIP
}

func run() error {
	zr, err := readSelfZip()
	if err != nil {
		return fmt.Errorf("reading payload: %w", err)
	}
	fsys := cleanfs.New(zr)

	// Read and parse AppManifest.json.
	manifestData, err := fs.ReadFile(fsys, "AppManifest.json")
	if err != nil {
		return fmt.Errorf("reading AppManifest.json: %w", err)
	}
	var manifest engine.AppManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("parsing AppManifest.json: %w", err)
	}
	manifest.PayloadFS = fsys

	// Read and parse WizardPages.json.
	pagesData, err := fs.ReadFile(fsys, "WizardPages.json")
	if err != nil {
		return fmt.Errorf("reading WizardPages.json: %w", err)
	}
	wizardPages, err := parsePages(pagesData, manifest, fsys)
	if err != nil {
		return fmt.Errorf("parsing WizardPages.json: %w", err)
	}

	engine.Run(manifest, wizardPages)
	return nil
}

// readSelfZip opens the running executable and returns a *zip.Reader over
// the ZIP data appended to the end of the PE. The file handle is intentionally
// not closed because zip entries are read lazily throughout the process lifetime.
func readSelfZip() (*zip.Reader, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(exe)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return zip.NewReader(f, info.Size())
}

// parsePages unmarshals WizardPages.json into a slice of WizardPage values.
// Each JSON element is either a bare string (page type name) or an object with
// a Type field plus extra settings.
func parsePages(data []byte, manifest engine.AppManifest, fsys fs.FS) ([]engine.WizardPage, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	result := make([]engine.WizardPage, 0, len(raw))
	for i, elem := range raw {
		var spec pageSpec
		if len(elem) > 0 && elem[0] == '"' {
			var typeName string
			if err := json.Unmarshal(elem, &typeName); err != nil {
				return nil, fmt.Errorf("page %d: %w", i, err)
			}
			spec = pageSpec{Type: typeName}
		} else {
			if err := json.Unmarshal(elem, &spec); err != nil {
				return nil, fmt.Errorf("page %d: %w", i, err)
			}
		}

		page, err := buildPage(spec, manifest, fsys)
		if err != nil {
			return nil, fmt.Errorf("page %d (%s): %w", i, spec.Type, err)
		}
		result = append(result, page)
	}
	return result, nil
}

// buildPage maps a pageSpec to the corresponding WizardPage implementation.
func buildPage(spec pageSpec, manifest engine.AppManifest, fsys fs.FS) (engine.WizardPage, error) {
	switch spec.Type {
	case "Welcome":
		return pages.Welcome(), nil
	case "Simple":
		p := &pages.Simple{
			PageTitle: spec.PageTitle,
			Fields:    spec.Fields,
		}
		if spec.ValidateFunc != "" {
			src, err := fs.ReadFile(fsys, spec.ValidateFunc)
			if err != nil {
				return nil, fmt.Errorf("reading ValidateFunc %q: %w", spec.ValidateFunc, err)
			}
			p.ValidateFunc = newValidateFunc(src)
		}
		return p, nil
	case "Process":
		if spec.Func == "" {
			return nil, fmt.Errorf("Process page requires Func")
		}
		src, err := fs.ReadFile(fsys, spec.Func)
		if err != nil {
			return nil, fmt.Errorf("reading Func %q: %w", spec.Func, err)
		}
		return &pages.Process{
			PageTitle: spec.PageTitle,
			Func:      newProcessFunc(src),
			Reentry:   spec.Reentry,
		}, nil
	case "InstallLocation":
		return pages.InstallLocation(), nil
	case "ServiceAccount":
		return pages.ServiceAccount(), nil
	case "AutoUpdates":
		return pages.AutoUpdates(), nil
	case "Review":
		return pages.Review(), nil
	case "Install":
		return pages.Install(manifest), nil
	case "License":
		if spec.File == "" {
			return nil, fmt.Errorf("License page requires File")
		}
		text, err := fs.ReadFile(fsys, spec.File)
		if err != nil {
			return nil, fmt.Errorf("reading File %q: %w", spec.File, err)
		}
		return pages.License(string(text)), nil
	case "Complete":
		return pages.Complete(), nil
	default:
		return nil, fmt.Errorf("unknown page type %q", spec.Type)
	}
}

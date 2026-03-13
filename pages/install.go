//go:build windows

package pages

import (
	"io"

	"github.com/9072997/thepipes/engine"
)

// Install returns the Install Progress wizard page.
func Install(manifest engine.AppManifest) engine.WizardPage {
	return &Process{
		PageTitle: "Installing " + manifest.Name,
		Func: func(log io.Writer, vars map[string]string) error {
			engine.Install(manifest, vars, log)
			return nil
		},
	}
}

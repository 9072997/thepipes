//go:build windows

package pages

import (
	"fmt"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// Complete returns the Setup Complete wizard page.
func Complete() engine.WizardPage { return &completePage{} }

type completePage struct{}

func (p *completePage) Title() string                 { return "Setup Complete" }
func (p *completePage) Skip(_ map[string]string) bool { return false }
func (p *completePage) Validate() error               { return nil }
func (p *completePage) Results() map[string]string    { return nil }

func (p *completePage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	msg := fmt.Sprintf("%s has been installed and the service has been started.", vars["name"])
	lbl := parent.Label(
		tk.Txt(msg),
		tk.Wraplength(620),
		tk.Anchor("w"),
		tk.Justify("left"),
		tk.Pady("8"),
	)
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))
}

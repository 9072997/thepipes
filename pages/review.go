//go:build windows

package pages

import (
	"fmt"
	"sort"
	"strings"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// Review returns the Review wizard page, which displays all accumulated settings.
func Review() engine.WizardPage { return &reviewPage{} }

type reviewPage struct{}

func (p *reviewPage) Title() string                 { return "Review Settings" }
func (p *reviewPage) Skip(_ map[string]string) bool { return false }
func (p *reviewPage) Validate() error               { return nil }
func (p *reviewPage) Results() map[string]string    { return nil }

func (p *reviewPage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	// Collect and sort keys (exclude hidden "." prefix keys).
	var keys []string
	for k := range vars {
		if !strings.HasPrefix(k, ".") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		lbl := parent.Label(tk.Txt("No settings to display."), tk.Anchor("w"))
		tk.Pack(lbl, tk.Side("top"))
		return
	}

	// Build a two-column grid.
	for i, k := range keys {
		v := vars[k]
		row := parent.Frame()
		tk.Grid(row, tk.Row(i), tk.Column(0), tk.Sticky("ew"), tk.Padx("4"), tk.Pady("2"))

		keyLbl := row.Label(
			tk.Txt(fmt.Sprintf("%-22s", k+":")),
			tk.Anchor("w"),
			tk.Font("TkFixedFont"),
		)
		tk.Pack(keyLbl, tk.Side("left"))

		valLbl := row.Label(
			tk.Txt(v),
			tk.Anchor("w"),
		)
		tk.Pack(valLbl, tk.Side("left"))
	}
}

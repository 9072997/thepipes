//go:build windows

package pages

import (
	"fmt"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// Welcome returns the Welcome wizard page.
func Welcome() engine.WizardPage { return &welcomePage{} }

type welcomePage struct{}

func (p *welcomePage) Title() string                 { return "Welcome" }
func (p *welcomePage) Skip(_ map[string]string) bool { return false }
func (p *welcomePage) Validate() error               { return nil }
func (p *welcomePage) Results() map[string]string    { return nil }

func (p *welcomePage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	info := "Application: " + vars["name"]
	if vars["version"] != "" {
		info += "\nVersion:     " + vars["version"]
	}
	if vars["publisher"] != "" {
		info += "\nPublisher:   " + vars["publisher"]
	}
	lbl := parent.Label(
		tk.Txt(info),
		tk.Anchor("w"),
		tk.Justify("left"),
		tk.Font("TkFixedFont"),
		tk.Pady("8"),
	)
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

	dirs := fmt.Sprintf(
		"Install directory:  %s\nData directory:     %s",
		vars["install"], vars["data"],
	)
	dirsLbl := parent.Label(
		tk.Txt(dirs),
		tk.Anchor("w"),
		tk.Justify("left"),
		tk.Font("TkFixedFont"),
		tk.Pady("4"),
	)
	tk.Pack(dirsLbl, tk.Side("top"), tk.Fill("x"))

	noteLbl := parent.Label(
		tk.Txt("Click Next to continue."),
		tk.Anchor("w"),
		tk.Pady("12"),
	)
	tk.Pack(noteLbl, tk.Side("top"), tk.Fill("x"))
}

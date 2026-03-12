//go:build windows

package pages

import (
	"fmt"
	"os"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// InstallLocation returns the Install Location wizard page.
func InstallLocation() engine.WizardPage { return &installLocationPage{} }

type installLocationPage struct {
	entry *tk.TEntryWidget
}

func (p *installLocationPage) Title() string                 { return "Choose Install Location" }
func (p *installLocationPage) Skip(_ map[string]string) bool { return false }

func (p *installLocationPage) Validate() error {
	if p.entry == nil {
		return nil
	}
	dir := p.entry.Textvariable()
	if dir == "" {
		return fmt.Errorf("install directory cannot be empty")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %v", dir, err)
	}
	// Write-test.
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return fmt.Errorf("cannot write to %s: %v", dir, err)
	}
	f.Close()
	os.Remove(f.Name())
	return nil
}

func (p *installLocationPage) Results() map[string]string {
	if p.entry == nil {
		return nil
	}
	dir := p.entry.Textvariable()
	if dir == "" {
		return nil
	}
	return map[string]string{"install": dir}
}

func (p *installLocationPage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	defaultDir := vars["install"]

	lbl := parent.Label(tk.Txt("Install directory:"), tk.Anchor("w"))
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

	rowF := parent.Frame()
	tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("4"))

	p.entry = rowF.TEntry(tk.Textvariable(defaultDir), tk.Width(50))
	tk.Pack(p.entry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

	browseBtn := rowF.TButton(tk.Txt("Browse..."), tk.Command(func() {
		result := tk.ChooseDirectory(tk.Title("Choose Install Directory"))
		if result != "" {
			p.entry.Configure(tk.Textvariable(result))
		}
	}))
	tk.Pack(browseBtn, tk.Side("left"), tk.Padx("4"))

	noteLbl := parent.Label(
		tk.Txt("At least 100 MB of free space is recommended."),
		tk.Anchor("w"),
		tk.Pady("8"),
	)
	tk.Pack(noteLbl, tk.Side("top"), tk.Fill("x"))
}

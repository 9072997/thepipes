//go:build windows

package pages

import (
	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// AutoUpdates returns the Auto Updates wizard page.
// The page is skipped when update_url is absent from vars.
func AutoUpdates() engine.WizardPage { return &autoUpdatesPage{} }

type autoUpdatesPage struct {
	enabled *tk.VariableOpt
}

func (p *autoUpdatesPage) Title() string { return "Automatic Updates" }

// Skip returns true when no update_url is configured - the page is never shown.
func (p *autoUpdatesPage) Skip(vars map[string]string) bool {
	return vars["update_url"] == ""
}

func (p *autoUpdatesPage) Validate() error { return nil }

func (p *autoUpdatesPage) Results() map[string]string {
	if p.enabled == nil {
		return map[string]string{"auto_updates": "true"}
	}
	val := "false"
	if p.enabled.Get() == "1" {
		val = "true"
	}
	return map[string]string{"auto_updates": val}
}

func (p *autoUpdatesPage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	// Default to enabled unless previously set to false.
	init := "1"
	if vars["auto_updates"] == "false" {
		init = "0"
	}
	p.enabled = tk.Variable(init)

	cb := parent.Checkbutton(
		tk.Txt("Automatically check for and install updates"),
		p.enabled,
	)
	tk.Pack(cb, tk.Side("top"), tk.Anchor("w"), tk.Pady("8"))

	info := parent.Label(
		tk.Txt("When enabled, the service will check "+vars["update_url"]+"\nonce per hour and apply updates automatically."),
		tk.Anchor("w"),
		tk.Justify("left"),
		tk.Wraplength(600),
	)
	tk.Pack(info, tk.Side("top"), tk.Fill("x"))
}

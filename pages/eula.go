//go:build windows

package pages

import (
	"errors"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// License returns a wizard page that displays a license agreement.
// The user must check an "I accept" checkbox before proceeding.
func License(text string) engine.WizardPage {
	return &licensePage{text: text}
}

type licensePage struct {
	text     string
	accepted *tk.VariableOpt
}

func (p *licensePage) Title() string                 { return "License Agreement" }
func (p *licensePage) Skip(_ map[string]string) bool { return false }
func (p *licensePage) Results() map[string]string    { return nil }

func (p *licensePage) Validate() error {
	if p.accepted == nil || p.accepted.Get() != "1" {
		return errors.New("you must accept the license agreement to continue")
	}
	return nil
}

func (p *licensePage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	// Scrolled read-only text widget for the License.
	textF := parent.Frame()
	tk.Pack(textF, tk.Side("top"), tk.Fill("both"), tk.Expand(true), tk.Pady("4"))

	scroll := textF.TScrollbar(tk.Orient("vertical"))
	txt := textF.Text(
		tk.Width(70),
		tk.Height(18),
		tk.Font("TkFixedFont"),
		tk.Wrap("word"),
		tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(scroll) }),
	)
	txt.Insert("end", p.text)
	txt.Configure(tk.State("disabled"))
	scroll.Configure(tk.Command(func(e *tk.Event) { e.Yview(txt) }))

	tk.Pack(txt, tk.Side("left"), tk.Fill("both"), tk.Expand(true))
	tk.Pack(scroll, tk.Side("right"), tk.Fill("y"))

	// "I accept" checkbox - Next is disabled until checked.
	init := "0"
	if p.accepted != nil && p.accepted.Get() == "1" {
		init = "1"
	}
	p.accepted = tk.Variable(init)
	nav.SetNext(init == "1")

	cb := parent.Checkbutton(
		tk.Txt("I accept the terms of the license agreement"),
		p.accepted,
		tk.Command(func() {
			nav.SetNext(p.accepted.Get() == "1")
		}),
	)
	tk.Pack(cb, tk.Side("top"), tk.Anchor("w"), tk.Pady("8"))
}

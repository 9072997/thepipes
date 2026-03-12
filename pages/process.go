//go:build windows

package pages

import (
	"bufio"
	"fmt"
	"io"
	"regexp"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

var pctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)

// Process is a declarative spec for a wizard page that runs a function and
// displays its output in a scrolling log with a progress bar.
//
// Lines written to the io.Writer appear in the listbox. If a line contains a
// percentage (e.g. "47%" or "12.5%"), the progress bar switches from
// indeterminate to determinate mode and updates to that value.
type Process struct {
	PageTitle string
	Func      func(log io.Writer, vars map[string]string)
	Reentry   bool // allow navigating back to this page after completion
}

func (p *Process) Title() string                 { return p.PageTitle }
func (p *Process) Skip(_ map[string]string) bool { return false }
func (p *Process) Validate() error               { return nil }
func (p *Process) Results() map[string]string    { return nil }

func (p *Process) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	nav.SetBack(false)
	nav.SetNext(false)

	// Progress bar + percentage label.
	progF := parent.Frame()
	tk.Pack(progF, tk.Side("top"), tk.Fill("x"), tk.Pady("4"))

	prog := progF.TProgressbar(tk.Length(500), tk.Mode("indeterminate"))
	tk.Pack(prog, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

	pctLbl := progF.Label(tk.Txt(""), tk.Width(6), tk.Anchor("e"))
	tk.Pack(pctLbl, tk.Side("left"), tk.Padx("4"))

	// Start in indeterminate mode.
	prog.Start()
	deterministic := false

	// Scrolled text widget for log lines.
	listF := parent.Frame()
	tk.Pack(listF, tk.Side("top"), tk.Fill("both"), tk.Expand(true), tk.Pady("4"))

	scroll := listF.TScrollbar(tk.Orient("vertical"))
	lb := listF.Text(
		tk.Width(70),
		tk.Height(15),
		tk.Font("TkFixedFont"),
		tk.Wrap("word"),
		tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(scroll) }),
	)
	lb.Configure(tk.State("disabled"))
	scroll.Configure(tk.Command(func(e *tk.Event) { e.Yview(lb) }))

	tk.Pack(lb, tk.Side("left"), tk.Fill("both"), tk.Expand(true))
	tk.Pack(scroll, tk.Side("right"), tk.Fill("y"))

	// Pipe: Func writes -> scanner reads.
	pr, pw := io.Pipe()

	// Work goroutine.
	go func() {
		p.Func(pw, vars)
		pw.Close()
	}()

	// Drain goroutine: read lines, post UI updates.
	go func() {
		scanner := bufio.NewScanner(pr)
		for scanner.Scan() {
			line := scanner.Text()
			tk.PostEvent(func() {
				lb.Configure(tk.State("normal"))
				lb.Insert("end", line+"\n")
				lb.See("end")
				lb.Configure(tk.State("disabled"))

				// Parse percentage from the line.
				matches := pctRe.FindAllStringSubmatch(line, -1)
				if len(matches) > 0 {
					last := matches[len(matches)-1][1]
					if !deterministic {
						prog.Stop()
						prog.Configure(tk.Mode("determinate"))
						deterministic = true
					}
					prog.Configure(tk.Value(last))
					pctLbl.Configure(tk.Txt(fmt.Sprintf("%s%%", last)))
				}
			}, false)
		}
		// Func returned and pipe closed - finalize.
		tk.PostEvent(func() {
			prog.Stop()
			if !deterministic {
				prog.Configure(tk.Mode("determinate"))
			}
			prog.Configure(tk.Value("100"))
			pctLbl.Configure(tk.Txt("100%"))
			if !p.Reentry {
				nav.DisableReentry()
			}
			nav.Advance()
		}, false)
	}()
}

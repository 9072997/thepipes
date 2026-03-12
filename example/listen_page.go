//go:build windows

package main

import (
	"fmt"
	"net"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// listenPage is a custom wizard page that collects the TCP listen address.
// It demonstrates how to implement engine.WizardPage directly in an installer.
type listenPage struct {
	entry *tk.TEntryWidget
}

func (p *listenPage) Title() string                 { return "Network" }
func (p *listenPage) Skip(_ map[string]string) bool { return false }

func (p *listenPage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	addr := ":80"
	if v := vars["listen"]; v != "" {
		addr = v
	}

	tk.Pack(
		parent.Label(tk.Txt("Listen address:"), tk.Anchor("w")),
		tk.Side("top"), tk.Fill("x"),
	)

	p.entry = parent.TEntry(tk.Textvariable(addr), tk.Width(20))
	tk.Pack(p.entry, tk.Side("top"), tk.Anchor("w"), tk.Pady("4"))

	tk.Pack(
		parent.Label(
			tk.Txt("Use :port to listen on all interfaces, or host:port for a specific one."),
			tk.Anchor("w"),
			tk.Pady("8"),
		),
		tk.Side("top"), tk.Fill("x"),
	)
}

func (p *listenPage) Validate() error {
	if p.entry == nil {
		return nil
	}
	addr := p.entry.Textvariable()
	if addr == "" {
		return fmt.Errorf("listen address cannot be empty")
	}
	if _, err := net.ResolveTCPAddr("tcp", addr); err != nil {
		return fmt.Errorf("invalid listen address %q: %v", addr, err)
	}
	return nil
}

func (p *listenPage) Results() map[string]string {
	if p.entry == nil {
		return map[string]string{"listen": ":80"}
	}
	return map[string]string{"listen": p.entry.Textvariable()}
}

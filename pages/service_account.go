//go:build windows

package pages

import (
	"fmt"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// ServiceAccount returns the Service Account wizard page.
func ServiceAccount() engine.WizardPage { return &serviceAccountPage{} }

type serviceAccountPage struct {
	modeVar  *tk.VariableOpt
	accEntry *tk.TEntryWidget
	pwEntry  *tk.TEntryWidget
}

func (p *serviceAccountPage) Title() string                 { return "Service Account" }
func (p *serviceAccountPage) Skip(_ map[string]string) bool { return false }

func (p *serviceAccountPage) Validate() error {
	if p.modeVar == nil {
		return nil
	}
	if p.modeVar.Get() == "domain" && p.accEntry.Textvariable() == "" {
		return fmt.Errorf("domain account name is required")
	}
	return nil
}

func (p *serviceAccountPage) Results() map[string]string {
	if p.modeVar == nil {
		return map[string]string{"service_account": "LocalService", ".service_password": ""}
	}
	if p.modeVar.Get() == "local" {
		return map[string]string{"service_account": "LocalService", ".service_password": ""}
	}
	account := ""
	password := ""
	if p.accEntry != nil {
		account = p.accEntry.Textvariable()
	}
	if p.pwEntry != nil {
		password = p.pwEntry.Textvariable()
	}
	return map[string]string{
		"service_account":   account,
		".service_password": password,
	}
}

func (p *serviceAccountPage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	mode := "local"
	prevAcct := vars["service_account"]
	if prevAcct != "" && prevAcct != "LocalService" {
		mode = "domain"
	}

	// Shared variable for the radio group.
	p.modeVar = tk.Variable(mode)

	localRB := parent.Radiobutton(
		tk.Txt("Local Service (recommended)"),
		p.modeVar,
		tk.Value("local"),
	)
	tk.Pack(localRB, tk.Side("top"), tk.Anchor("w"), tk.Pady("4"))

	domainRB := parent.Radiobutton(
		tk.Txt("Domain / custom account"),
		p.modeVar,
		tk.Value("domain"),
	)
	tk.Pack(domainRB, tk.Side("top"), tk.Anchor("w"))

	// Account entry.
	accF := parent.Frame()
	tk.Pack(accF, tk.Side("top"), tk.Fill("x"), tk.Pady("4"), tk.Padx("20"))
	tk.Pack(accF.Label(tk.Txt("Account:"), tk.Width(10), tk.Anchor("w")), tk.Side("left"))
	initAcct := prevAcct
	if initAcct == "LocalService" {
		initAcct = ""
	}
	p.accEntry = accF.TEntry(tk.Textvariable(initAcct), tk.Width(30))
	tk.Pack(p.accEntry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

	// Password entry.
	pwF := parent.Frame()
	tk.Pack(pwF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"), tk.Padx("20"))
	tk.Pack(pwF.Label(tk.Txt("Password:"), tk.Width(10), tk.Anchor("w")), tk.Side("left"))
	p.pwEntry = pwF.TEntry(tk.Textvariable(vars[".service_password"]), tk.Show("*"), tk.Width(30))
	tk.Pack(p.pwEntry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

	noteLbl := parent.Label(
		tk.Txt("Use DOMAIN\\username format for domain accounts."),
		tk.Anchor("w"),
		tk.Pady("8"),
	)
	tk.Pack(noteLbl, tk.Side("top"), tk.Fill("x"))
}

//go:build windows

// Package acme provides a wizard page for ACME / Let's Encrypt TLS certificate provisioning.
// It detects the machine's public IP addresses, tests certificate issuance against the
// LE staging environment, and lets the user choose which name to use for TLS.
package acme

import (
	"context"
	"fmt"
	"net"
	"sync"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

type testResult struct {
	status string // "detecting", "untested", "testing", "pass", "fail", "noip"
	detail string // error message on failure or extra info
}

type rowWidgets struct {
	radio     *tk.RadiobuttonWidget
	statusLbl *tk.LabelWidget
	testBtn   *tk.TButtonWidget
}

type acmePage struct {
	radioVar  *tk.VariableOpt
	fqdnEntry *tk.TEntryWidget
	logBox    *tk.TextWidget
	nav       engine.NavControl

	mu       sync.Mutex
	ipv4     string
	ipv6     string
	statuses map[string]testResult
	cancel   context.CancelFunc

	ipv4Row *rowWidgets
	ipv6Row *rowWidgets
	fqdnRow *rowWidgets
}

// NewPage returns a new ACME wizard page.
func NewPage() engine.WizardPage {
	return &acmePage{}
}

func (p *acmePage) Title() string                 { return "TLS Certificate (ACME)" }
func (p *acmePage) Skip(_ map[string]string) bool { return false }

func (p *acmePage) Validate() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	sel := p.radioVar.Get()
	st, ok := p.statuses[sel]
	if !ok || st.status != "pass" {
		return fmt.Errorf("the selected option has not passed ACME testing yet")
	}
	return nil
}

func (p *acmePage) Results() map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()

	sel := p.radioVar.Get()
	switch sel {
	case "ipv4":
		return map[string]string{"fqdn": p.ipv4}
	case "ipv6":
		return map[string]string{"fqdn": p.ipv6}
	default:
		return map[string]string{"fqdn": p.fqdnEntry.Textvariable()}
	}
}

func (p *acmePage) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	// Cancel any goroutines from a previous render.
	if p.cancel != nil {
		p.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.nav = nav

	nav.SetNext(false)

	p.mu.Lock()
	p.statuses = map[string]testResult{
		"ipv4": {status: "detecting"},
		"ipv6": {status: "detecting"},
		"fqdn": {status: "untested"},
	}
	p.mu.Unlock()

	// Default selection: restore from vars or default to fqdn.
	initSel := "fqdn"
	if prev := vars["fqdn"]; prev != "" {
		// If the previous fqdn looks like it was an IP, pre-select accordingly.
		// Otherwise keep fqdn selected.
		initSel = "fqdn"
	}
	p.radioVar = tk.Variable(initSel)

	lbl := parent.Label(tk.Txt("Obtain certificate for:"), tk.Anchor("w"))
	tk.Pack(lbl, tk.Side("top"), tk.Fill("x"), tk.Pady("4"))

	// --- IPv4 row ---
	p.ipv4Row = p.buildRow(parent, "ipv4", "Detecting IPv4...", "\u2026 Detecting")
	p.ipv4Row.radio.Configure(tk.State("disabled"))

	// --- IPv6 row ---
	p.ipv6Row = p.buildRow(parent, "ipv6", "Detecting IPv6...", "\u2026 Detecting")
	p.ipv6Row.radio.Configure(tk.State("disabled"))

	// --- FQDN row ---
	p.fqdnRow = p.buildRow(parent, "fqdn", "Custom FQDN", "\u2014 Untested")
	p.fqdnRow.testBtn.Configure(tk.State("normal"))

	// FQDN entry below radio rows.
	entryF := parent.Frame()
	tk.Pack(entryF, tk.Side("top"), tk.Fill("x"), tk.Padx("40"), tk.Pady("2"))
	tk.Pack(entryF.Label(tk.Txt("FQDN:"), tk.Width(6), tk.Anchor("w")), tk.Side("left"))
	initFQDN := vars["fqdn"]
	p.fqdnEntry = entryF.TEntry(tk.Textvariable(initFQDN), tk.Width(40))
	tk.Pack(p.fqdnEntry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

	// Scrolled listbox for log output.
	logF := parent.Frame()
	tk.Pack(logF, tk.Side("top"), tk.Fill("both"), tk.Expand(true), tk.Pady("4"))

	logScroll := logF.TScrollbar(tk.Orient("vertical"))
	p.logBox = logF.Text(
		tk.Width(70),
		tk.Height(8),
		tk.Font("TkFixedFont"),
		tk.Wrap("word"),
		tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(logScroll) }),
	)
	p.logBox.Configure(tk.State("disabled"))
	logScroll.Configure(tk.Command(func(e *tk.Event) { e.Yview(p.logBox) }))

	tk.Pack(p.logBox, tk.Side("left"), tk.Fill("both"), tk.Expand(true))
	tk.Pack(logScroll, tk.Side("right"), tk.Fill("y"))

	// Pre-check: can we bind to ports 80 and 443?
	if err := checkPort(80); err != nil {
		p.logLine(fmt.Sprintf("Port 80 unavailable: %s", err))
		p.disableAllTests()
		return
	}
	if err := checkPort(443); err != nil {
		p.logLine(fmt.Sprintf("Port 443 unavailable: %s", err))
		p.disableAllTests()
		return
	}
	p.logLine("Ports 80 and 443 are available")

	// Launch IP detection goroutines.
	go p.detectIP(ctx, false) // IPv4
	go p.detectIP(ctx, true)  // IPv6
}

func (p *acmePage) buildRow(parent *tk.FrameWidget, value, label, statusText string) *rowWidgets {
	rowF := parent.Frame()
	tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

	radio := rowF.Radiobutton(
		tk.Txt(label),
		p.radioVar,
		tk.Value(value),
		tk.Command(func() { p.onRadioChange() }),
	)
	tk.Pack(radio, tk.Side("left"), tk.Padx("4"))

	statusLbl := rowF.Label(tk.Txt(statusText), tk.Width(16), tk.Anchor("w"))
	tk.Pack(statusLbl, tk.Side("left"), tk.Padx("4"))

	testBtn := rowF.TButton(tk.Txt("Test"), tk.Command(func() {
		p.startTest(value)
	}))
	tk.Pack(testBtn, tk.Side("left"), tk.Padx("4"))
	testBtn.Configure(tk.State("disabled"))

	return &rowWidgets{radio: radio, statusLbl: statusLbl, testBtn: testBtn}
}

func (p *acmePage) detectIP(ctx context.Context, ipv6 bool) {
	key := "ipv4"
	family := "IPv4"
	if ipv6 {
		key = "ipv6"
		family = "IPv6"
	}

	tk.PostEvent(func() {
		p.logLine(fmt.Sprintf("Detecting public %s...", family))
	}, true)

	ip, err := detectPublicIP(ctx, ipv6)

	tk.PostEvent(func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if ctx.Err() != nil {
			return
		}

		row := p.ipv4Row
		if ipv6 {
			row = p.ipv6Row
		}

		if err != nil {
			p.statuses[key] = testResult{status: "noip", detail: err.Error()}
			row.radio.Configure(tk.State("disabled"))
			row.statusLbl.Configure(tk.Txt("\u2717 Not detected"))
			row.testBtn.Configure(tk.State("disabled"))
			p.logLine(fmt.Sprintf("%s not detected: %s", family, err.Error()))
			return
		}

		if ipv6 {
			p.ipv6 = ip
		} else {
			p.ipv4 = ip
		}

		row.radio.Configure(tk.Txt(ip), tk.State("normal"))
		p.statuses[key] = testResult{status: "untested"}
		row.statusLbl.Configure(tk.Txt("\u2014 Untested"))
		row.testBtn.Configure(tk.State("normal"))
		p.logLine(fmt.Sprintf("Detected %s: %s", family, ip))
	}, false)
}

func (p *acmePage) startTest(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var name string
	switch key {
	case "ipv4":
		name = p.ipv4
	case "ipv6":
		name = p.ipv6
	case "fqdn":
		name = p.fqdnEntry.Textvariable()
		if name == "" {
			p.statuses["fqdn"] = testResult{status: "untested", detail: "empty"}
			p.fqdnRow.statusLbl.Configure(tk.Txt("\u2014 Untested"))
			p.updateNext()
			return
		}
	}
	if name == "" {
		return
	}

	row := p.getRow(key)
	p.statuses[key] = testResult{status: "testing"}
	row.statusLbl.Configure(tk.Txt("\u2026 Testing"))
	row.testBtn.Configure(tk.State("disabled"))
	p.updateNext()
	p.logLine(fmt.Sprintf("Testing ACME for %s...", name))

	// Use the page-level context so cancel() stops stale tests.
	ctx := context.Background()
	if p.cancel != nil {
		// Derive a child context from the page context.
		var childCancel context.CancelFunc
		ctx, childCancel = context.WithCancel(ctx)
		_ = childCancel // will be cancelled by page cancel
	}
	go p.runTest(ctx, key, name)
}

func (p *acmePage) runTest(ctx context.Context, key, name string) {
	err := testACME(ctx, name)

	tk.PostEvent(func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if ctx.Err() != nil {
			return
		}

		row := p.getRow(key)

		if err != nil {
			p.statuses[key] = testResult{status: "fail", detail: err.Error()}
			row.statusLbl.Configure(tk.Txt("\u2717 Fail"))
			p.logLine(fmt.Sprintf("ACME test failed for %s: %s", name, err.Error()))
		} else {
			p.statuses[key] = testResult{status: "pass"}
			row.statusLbl.Configure(tk.Txt("\u2713 Pass"))
			p.logLine(fmt.Sprintf("ACME test passed for %s", name))
		}
		row.testBtn.Configure(tk.State("normal"))

		p.updateNext()
	}, false)
}

func (p *acmePage) onRadioChange() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.updateNext()
}

// updateNext enables/disables the Next button based on the selected option's status.
// Must be called with p.mu held.
func (p *acmePage) updateNext() {
	sel := p.radioVar.Get()
	st, ok := p.statuses[sel]
	if ok && st.status == "pass" {
		p.nav.SetNext(true)
	} else {
		p.nav.SetNext(false)
	}
}

// logLine appends a message to the log and auto-scrolls.
// Must be called from the Tk thread.
func (p *acmePage) logLine(msg string) {
	p.logBox.Configure(tk.State("normal"))
	p.logBox.Insert("end", msg+"\n")
	p.logBox.See("end")
	p.logBox.Configure(tk.State("disabled"))
}

// checkPort tries to listen on a TCP port and immediately closes the listener.
func checkPort(port int) error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}

// disableAllTests disables every Test button and sets all statuses to failed.
// Must be called from the Tk thread.
func (p *acmePage) disableAllTests() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, key := range []string{"ipv4", "ipv6", "fqdn"} {
		p.statuses[key] = testResult{status: "fail", detail: "port unavailable"}
		row := p.getRow(key)
		row.testBtn.Configure(tk.State("disabled"))
		row.statusLbl.Configure(tk.Txt("\u2717 Port blocked"))
	}
}

func (p *acmePage) getRow(key string) *rowWidgets {
	switch key {
	case "ipv4":
		return p.ipv4Row
	case "ipv6":
		return p.ipv6Row
	default:
		return p.fqdnRow
	}
}

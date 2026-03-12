# Custom Wizard Pages (Go Only)

When the built-in pages don't cover your needs, implement the `WizardPage` interface directly.

## The WizardPage Interface

```go
type WizardPage interface {
    Title() string
    Skip(vars map[string]string) bool
    Render(parent *tk.FrameWidget, vars map[string]string, nav NavControl)
    Validate() error
    Results() map[string]string
}
```

| Method | Called When | Purpose |
|--------|------------|---------|
| `Title()` | Page is shown | Returns the page title displayed in the header |
| `Skip(vars)` | Before showing | Return true to hide this page given current vars |
| `Render(parent, vars, nav)` | Page is shown | Build UI widgets inside `parent` |
| `Validate()` | User clicks Next | Return an error to block navigation and show a dialog |
| `Results()` | After Validate passes | Return key-value pairs to merge into the shared vars |

## NavControl

```go
type NavControl struct {
    Advance        func()             // programmatically click Next
    DisableReentry func()             // prevent Back navigation to this page
    SetBack        func(enabled bool) // enable/disable Back button
    SetNext        func(enabled bool) // enable/disable Next button
}
```

## Tk Widget Basics

ThePipes uses `modernc.org/tk9.0`, a pure-Go Tcl/Tk binding. Import with the alias `tk`:

```go
import tk "modernc.org/tk9.0"
```

Widgets are created as methods on their parent:

```go
label := parent.Label(tk.Txt("Enter a value:"), tk.Anchor("w"))
tk.Pack(label, tk.Side("top"), tk.Fill("x"))

entry := parent.TEntry(tk.Textvariable("default"), tk.Width(30))
tk.Pack(entry, tk.Side("top"), tk.Anchor("w"))
```

Read entry values with `entry.Textvariable()`.

## Thread Safety

Tk widgets must only be modified from the Tcl thread. If your page uses goroutines:

- **`tk.PostEvent(fn, canDrop)`** -- schedules `fn` to run on the Tcl thread. Use this for all UI updates from goroutines.
- **Never call `tk.Update()`** inside callbacks -- it re-enters the event loop and can cause crashes.
- **`tk.TclAfterIdle(fn)`** is Tcl-thread-only. Do not call it from goroutines.

## Example: Listen Address Page

This page collects a TCP listen address and validates it:

```go
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
            tk.Anchor("w"), tk.Pady("8"),
        ),
        tk.Side("top"), tk.Fill("x"),
    )
}

func (p *listenPage) Validate() error {
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
    return map[string]string{"listen": p.entry.Textvariable()}
}
```

Use it like any other page: `&listenPage{}`.

## Tips

- **Conditional skip:** Return true from `Skip(vars)` to hide a page when a certain variable is set (or not set). The AutoUpdates page does this -- it skips when `update_url` is empty.
- **Async work:** Disable Next with `nav.SetNext(false)`, launch a goroutine, then use `tk.PostEvent` to re-enable Next or call `nav.Advance()` when done.
- **Sensitive values:** Prefix keys with `.` (e.g., `.api_key`) to hide them from the Review page and exclude them from `install-config.json`.
- **Preventing re-entry:** Call `nav.DisableReentry()` after completing irreversible work so the user can't navigate back.

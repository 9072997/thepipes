//go:build windows

package pages

import (
	"fmt"
	"strconv"
	"strings"

	tk "modernc.org/tk9.0"

	"github.com/9072997/thepipes/engine"
)

// Field describes one labeled input row on a Simple page.
type Field struct {
	Key     string // key in the results map
	Label   string // human-readable label displayed next to the widget
	Type    string // magic value (TEXT/UNSIGNED/INT/FLOAT/FILE/FOLDER/BOOL) or "a/b/c" for dropdown
	Default string // value pre-filled when vars[Key] is absent
}

// Simple is a declarative spec for a wizard page composed of labeled input fields.
// Populate PageTitle and Fields, then pass *Simple directly as an engine.WizardPage.
//
// PageTitle is the page title. It is intentionally not called Title to avoid a
// conflict with the Title() method required by engine.WizardPage.
// ValidateFunc is the optional hook field - named ValidateFunc (not Validate) to
// avoid a conflict with the Validate() method required by engine.WizardPage.
type Simple struct {
	PageTitle    string // page title
	Fields       []Field
	ValidateFunc func(map[string]string) error // optional extra validation, run after built-in checks

	// unexported widget state - populated during Render, read in Validate/Results.
	widgets []simpleWidget
}

// simpleWidget is a tagged union; exactly one pointer is non-nil.
type simpleWidget struct {
	entry *tk.TEntryWidget    // TEXT, UNSIGNED, INT, FLOAT, FILE, FOLDER
	bVar  *tk.VariableOpt     // BOOL (stores "1"/"0" internally)
	combo *tk.TComboboxWidget // dropdown (a/b/c)
}

func (s *Simple) Title() string                 { return s.PageTitle }
func (s *Simple) Skip(_ map[string]string) bool { return false }

func (s *Simple) Render(parent *tk.FrameWidget, vars map[string]string, nav engine.NavControl) {
	s.widgets = make([]simpleWidget, len(s.Fields))

	for i, f := range s.Fields {
		val := f.Default
		if v := vars[f.Key]; v != "" {
			val = v
		}

		if strings.Contains(f.Type, "/") {
			// Dropdown - split on "/" to get options.
			options := strings.Split(f.Type, "/")
			if val == "" {
				val = options[0]
			}

			lbl := parent.Label(tk.Txt(f.Label+":"), tk.Anchor("w"))
			tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

			rowF := parent.Frame()
			tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

			combo := rowF.TCombobox(
				tk.State("readonly"),
				tk.Values(options),
				tk.Textvariable(val),
			)
			tk.Pack(combo, tk.Side("left"))
			s.widgets[i] = simpleWidget{combo: combo}
			continue
		}

		switch f.Type {
		case "BOOL":
			// Checkbutton carries its own label - no separate label row.
			init := "0"
			if val == "true" {
				init = "1"
			}
			bVar := tk.Variable(init)

			rowF := parent.Frame()
			tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

			cb := rowF.Checkbutton(tk.Txt(f.Label), bVar)
			tk.Pack(cb, tk.Side("left"))
			s.widgets[i] = simpleWidget{bVar: bVar}

		case "FILE":
			lbl := parent.Label(tk.Txt(f.Label+":"), tk.Anchor("w"))
			tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

			rowF := parent.Frame()
			tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

			entry := rowF.TEntry(tk.Textvariable(val))
			tk.Pack(entry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

			browseBtn := rowF.TButton(tk.Txt("Browse..."), tk.Command(func() {
				files := tk.GetOpenFile()
				if len(files) > 0 {
					entry.Configure(tk.Textvariable(files[0]))
				}
			}))
			tk.Pack(browseBtn, tk.Side("left"), tk.Padx("4"))
			s.widgets[i] = simpleWidget{entry: entry}

		case "FOLDER":
			lbl := parent.Label(tk.Txt(f.Label+":"), tk.Anchor("w"))
			tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

			rowF := parent.Frame()
			tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

			entry := rowF.TEntry(tk.Textvariable(val))
			tk.Pack(entry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))

			browseBtn := rowF.TButton(tk.Txt("Browse..."), tk.Command(func() {
				result := tk.ChooseDirectory()
				if result != "" {
					entry.Configure(tk.Textvariable(result))
				}
			}))
			tk.Pack(browseBtn, tk.Side("left"), tk.Padx("4"))
			s.widgets[i] = simpleWidget{entry: entry}

		default:
			// TEXT, UNSIGNED, INT, FLOAT - plain text entry.
			lbl := parent.Label(tk.Txt(f.Label+":"), tk.Anchor("w"))
			tk.Pack(lbl, tk.Side("top"), tk.Fill("x"))

			rowF := parent.Frame()
			tk.Pack(rowF, tk.Side("top"), tk.Fill("x"), tk.Pady("2"))

			entry := rowF.TEntry(tk.Textvariable(val))
			tk.Pack(entry, tk.Side("left"), tk.Fill("x"), tk.Expand(true))
			s.widgets[i] = simpleWidget{entry: entry}
		}
	}
}

func (s *Simple) Validate() error {
	if s.widgets == nil {
		return nil
	}

	for i, f := range s.Fields {
		w := s.widgets[i]
		var v string
		switch {
		case w.entry != nil:
			v = w.entry.Textvariable()
		case w.bVar != nil:
			continue // BOOL needs no numeric validation
		case w.combo != nil:
			continue // dropdown values are always valid
		}

		if v == "" {
			continue
		}

		var err error
		switch f.Type {
		case "UNSIGNED":
			_, err = strconv.ParseUint(v, 10, 64)
		case "INT":
			_, err = strconv.ParseInt(v, 10, 64)
		case "FLOAT":
			_, err = strconv.ParseFloat(v, 64)
		}
		if err != nil {
			return fmt.Errorf("%s: %q is not a valid %s value", f.Label, v, strings.ToLower(f.Type))
		}
	}

	if s.ValidateFunc != nil {
		return s.ValidateFunc(s.results())
	}
	return nil
}

func (s *Simple) Results() map[string]string {
	if s.widgets == nil {
		return nil
	}
	return s.results()
}

// results collects widget values into a map without calling the custom Validate hook.
func (s *Simple) results() map[string]string {
	out := make(map[string]string, len(s.Fields))
	for i, f := range s.Fields {
		w := s.widgets[i]
		switch {
		case w.entry != nil:
			out[f.Key] = w.entry.Textvariable()
		case w.bVar != nil:
			if w.bVar.Get() == "1" {
				out[f.Key] = "true"
			} else {
				out[f.Key] = "false"
			}
		case w.combo != nil:
			out[f.Key] = w.combo.Textvariable()
		}
	}
	return out
}

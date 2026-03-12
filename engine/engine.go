//go:build windows

package engine

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/9072997/thepipes/internal/cleanfs"
	tk "modernc.org/tk9.0"
)

// AppManifest describes an application managed by ThePipes.
type AppManifest struct {
	Name      string
	Publisher string
	Version   string
	Command   string            // e.g. `myapp.exe --port %port%`
	Shortcuts map[string]string // name -> URL or relative path
	UpdateURL string            // optional update URL
	PayloadFS fs.FS
}

// WizardPage is a single page in the setup wizard.
type WizardPage interface {
	Title() string
	// Skip returns true if this page should be omitted given prior results.
	Skip(vars map[string]string) bool
	// Render populates parent with this page's widgets.
	Render(parent *tk.FrameWidget, vars map[string]string, nav NavControl)
	// Validate returns an error if the page's input is invalid.
	Validate() error
	// Results returns the key-value pairs produced by this page.
	Results() map[string]string
}

// NavControl gives a page the ability to control wizard navigation.
type NavControl struct {
	Advance        func()
	DisableReentry func()
	SetBack        func(enabled bool)
	SetNext        func(enabled bool)
}

// Run dispatches to the appropriate operating mode based on os.Args,
// then falls through to the setup wizard.
func Run(manifest AppManifest, pages []WizardPage) {
	manifest.PayloadFS = cleanfs.New(manifest.PayloadFS)
	args := os.Args[1:]
	if len(args) >= 1 {
		switch args[0] {
		case "--service":
			if err := serviceMode(manifest); err != nil {
				showFatalError(fmt.Sprintf("Service error: %v", err))
			}
			return
		case "--update":
			iDir := getInstallDir(manifest.Name)
			if err := updateMode(manifest, iDir); err != nil {
				fmt.Fprintf(os.Stderr, "update failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--apply-update":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "--apply-update requires a directory argument")
				os.Exit(1)
			}
			if err := applyUpdateMode(manifest, args[1]); err != nil {
				fmt.Fprintf(os.Stderr, "apply-update failed: %v\n", err)
				os.Exit(1)
			}
			return
		case "--uninstall":
			uninstallMode(manifest)
			return
		case "--cleanup":
			if len(args) < 2 {
				return
			}
			cleanupMode(args[1])
			return
		}
	}

	runWizard(manifest, pages)
}

func runWizard(manifest AppManifest, pages []WizardPage) {
	if !isAdmin() {
		selfElevate()
	}

	vars := BuiltinVars(manifest)

	// Find the first non-skipped page.
	current := -1
	for i := range pages {
		if !pages[i].Skip(vars) {
			current = i
			break
		}
	}
	if current == -1 {
		return
	}
	history := []int{current}
	noReentry := map[int]bool{}

	// Configure the main window.
	tk.App.WmTitle(manifest.Name + " Setup")
	tk.App.SetResizable(false, false)
	tk.WmGeometry(tk.App, "700x520")

	// Try to load an icon from PayloadFS.
	loadWindowIcon(manifest)

	// Top-level frame.
	mainF := tk.App.Frame()
	tk.Pack(mainF, tk.Fill("both"), tk.Expand(true))

	// Header (white background with title).
	headerF := mainF.Frame(tk.Background("white"))
	tk.Pack(headerF, tk.Side("top"), tk.Fill("x"))

	titleLbl := headerF.Label(
		tk.Background("white"),
		tk.Font("TkHeadingFont"),
		tk.Anchor("w"),
		tk.Padx("8"),
		tk.Pady("6"),
	)
	tk.Pack(titleLbl, tk.Side("top"), tk.Fill("x"))

	// Top separator.
	sep1 := mainF.Frame(tk.Height(1), tk.Background("gray70"))
	tk.Pack(sep1, tk.Side("top"), tk.Fill("x"))

	// Footer packed before separator so the separator sits above the buttons.
	footerF := mainF.Frame()
	tk.Pack(footerF, tk.Side("bottom"), tk.Fill("x"), tk.Padx("8"), tk.Pady("6"))

	sep2 := mainF.Frame(tk.Height(1), tk.Background("gray70"))
	tk.Pack(sep2, tk.Side("bottom"), tk.Fill("x"))

	cancelBtn := footerF.TButton(tk.Txt("Cancel"), tk.Command(func() {
		os.Exit(0)
	}))
	tk.Pack(cancelBtn, tk.Side("left"))
	_ = cancelBtn

	navF := footerF.Frame()
	tk.Pack(navF, tk.Side("right"))

	var backBtn, nextBtn *tk.TButtonWidget
	var contentF *tk.FrameWidget
	var goNext func()

	// backBlocked returns true if the Back button should be hard-disabled
	// because there is no safe page to go back to.
	backBlocked := func() bool {
		if len(history) <= 1 {
			return true
		}
		return noReentry[history[len(history)-2]]
	}

	var showPage func(idx int)
	showPage = func(idx int) {
		current = idx
		page := pages[idx]

		// Forward-navigating onto a previously locked page clears the flag.
		delete(noReentry, idx)

		titleLbl.Configure(tk.Txt(page.Title()))

		// Recreate content frame.
		if contentF != nil {
			tk.Destroy(contentF)
		}
		contentF = mainF.Frame()
		tk.Pack(contentF, tk.Side("top"), tk.Fill("both"), tk.Expand(true),
			tk.Padx("12"), tk.Pady("8"))

		// Set default Back state before Render so page can override.
		if backBlocked() {
			backBtn.Configure(tk.State("disabled"))
		} else {
			backBtn.Configure(tk.State("normal"))
		}

		// Reset Next to enabled before Render.
		nextBtn.Configure(tk.State("normal"))

		// Build NavControl for this page.
		nav := NavControl{
			DisableReentry: func() {
				noReentry[current] = true
				backBtn.Configure(tk.State("disabled"))
			},
			Advance: func() {
				tk.TclAfterIdle(goNext)
			},
			SetBack: func(enabled bool) {
				if enabled && backBlocked() {
					return // can't re-enable if back target is noReentry
				}
				if enabled {
					backBtn.Configure(tk.State("normal"))
				} else {
					backBtn.Configure(tk.State("disabled"))
				}
			},
			SetNext: func(enabled bool) {
				if enabled {
					nextBtn.Configure(tk.State("normal"))
				} else {
					nextBtn.Configure(tk.State("disabled"))
				}
			},
		}

		page.Render(contentF, vars, nav)

		// Next becomes Finish on the last page.
		hasNext := false
		for i := idx + 1; i < len(pages); i++ {
			if !pages[i].Skip(vars) {
				hasNext = true
				break
			}
		}
		if hasNext {
			nextBtn.Configure(tk.Txt("Next >"))
		} else {
			nextBtn.Configure(tk.Txt("Finish"))
		}
	}

	goNext = func() {
		page := pages[current]
		if err := page.Validate(); err != nil {
			showError(err.Error())
			return
		}
		for k, v := range page.Results() {
			vars[k] = v
		}

		// Find next non-skipped page.
		next := -1
		for i := current + 1; i < len(pages); i++ {
			if !pages[i].Skip(vars) {
				next = i
				break
			}
		}
		if next == -1 {
			tk.Destroy(tk.App)
			return
		}
		history = append(history, next)
		showPage(next)
	}

	backBtn = navF.TButton(tk.Txt("< Back"), tk.Command(func() {
		if len(history) > 1 {
			history = history[:len(history)-1]
			showPage(history[len(history)-1])
		}
	}))
	tk.Pack(backBtn, tk.Side("left"), tk.Padx("4"))

	nextBtn = navF.TButton(tk.Txt("Next >"), tk.Command(func() { goNext() }))
	tk.Pack(nextBtn, tk.Side("left"))

	showPage(current)
	tk.App.Wait()
}

// showError shows a modal error message box.
func showError(msg string) {
	tk.MessageBox(
		tk.Icon("error"),
		tk.Msg(msg),
		tk.Title("Error"),
	)
}

// showFatalError shows an error message box and exits.
func showFatalError(msg string) {
	showError(msg)
	os.Exit(1)
}

// installBinaryPath returns the path where the installer binary lives after install.
func installBinaryPath(manifest AppManifest) string {
	return filepath.Join(getInstallDir(manifest.Name), binaryName(manifest.Name))
}

// loadWindowIcon searches for an icon file in PayloadFS and sets it as the window icon.
// Supported formats: png, ico, gif, jpg, jpeg, bmp, svg, tiff, tif
func loadWindowIcon(manifest AppManifest) {
	// Try common icon formats in order of preference.
	formats := []string{"png", "ico", "gif", "jpg", "jpeg", "bmp", "svg", "tiff", "tif"}

	for _, ext := range formats {
		iconPath := "installer." + ext
		data, err := fs.ReadFile(manifest.PayloadFS, iconPath)
		if err != nil {
			continue // File doesn't exist, try next format.
		}

		// Create a photo image from the data.
		img := tk.NewPhoto(tk.Data(data))

		// Set it as the window icon for this window and all future toplevels.
		tk.App.IconPhoto(tk.DefaultIcon(), img)
		return
	}
}

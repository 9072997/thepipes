//go:build windows

// Command installer is a usage example showing how to build a ThePipes installer.
//
// Build the example server first, then build this installer:
//
//	cd payload ; go build
//	go build -ldflags="-H windowsgui"
package main

import (
	"embed"
	"fmt"
	"io"
	"time"

	"github.com/9072997/thepipes/engine"
	"github.com/9072997/thepipes/pages"
	"github.com/9072997/thepipes/pages/acme"
)

//go:embed payload
var payload embed.FS

func main() {
	manifest := engine.AppManifest{
		Name:      "My App",
		Publisher: "My Company",
		Version:   "1.0.0",
		Command:   `payload.exe --flag="%message%"`,
		Shortcuts: map[string]string{
			// %listen% is already a colon-prefixed port like ":80", so no
			// extra colon between "localhost" and the variable.
			"My App":         "http://localhost%listen%",
			"Install Config": `%data%\install-config.json`,
		},
		UpdateURL: "https://downloads.example.com/my-app-setup.exe",
		PayloadFS: payload,
	}

	engine.Run(manifest, []engine.WizardPage{
		pages.Welcome(),
		pages.License("if it breaks\nyou get to keep both pieces"),
		&listenPage{},
		&pages.Simple{
			PageTitle: "Application Settings",
			Fields: []pages.Field{
				{Key: "message", Label: "Welcome message", Type: "TEXT", Default: "Hello from My App"},
			},
		},
		acme.NewPage(),
		&pages.Process{
			PageTitle: "Doing Something",
			Func:      doSomething,
		},
		pages.InstallLocation(),
		pages.ServiceAccount(),
		pages.AutoUpdates(),
		pages.Review(),
		pages.Install(manifest),
		pages.Complete(),
	})
}

func doSomething(log io.Writer, vars map[string]string) error {
	// these get a spinner
	fmt.Fprintln(log, "Hi")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "We're getting everything ready for you")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "This might take several minutes")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "Don't turn off your PC")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "Leave everything to us")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "Windows stays up to date to help protect you in an online world")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "Making sure your apps are good to go")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "Almost there")
	time.Sleep(time.Second)
	fmt.Fprintln(log, "It's taking a bit longer than expected, but we'll get there as fast as we can")
	time.Sleep(time.Second)

	// if our lines contain percentages, the progress bar will pick it up
	for i := 0; i <= 100; i += 10 {
		fmt.Fprintf(log, "We are %d%% done doing something\n", i)
		time.Sleep(time.Second)
		fmt.Fprintln(log, "Example line")
		time.Sleep(time.Second)
	}
	return nil
}

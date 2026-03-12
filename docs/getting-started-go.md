# Getting Started with Go

## Prerequisites

- Go 1.22+
- Windows build target (`GOOS=windows`)
- Build with `-ldflags="-H windowsgui"` to suppress the console window

## Project Layout

```
myapp-installer/
  cmd/installer/
    main.go          # AppManifest + page list + engine.Run()
  payload/
    myapp.exe        # your application binary
    config.toml      # any other files to install
  go.mod
```

## Minimal Example

```go
//go:build windows

package main

import (
    "embed"

    "github.com/9072997/thepipes/engine"
    "github.com/9072997/thepipes/pages"
)

//go:embed payload
var payload embed.FS

func main() {
    manifest := engine.AppManifest{
        Name:      "My App",
        Publisher: "My Company",
        Version:   "1.0.0",
        Command:   `myapp.exe --config "%data%\config.toml"`,
        Shortcuts: map[string]string{
            "My App": "http://localhost:8080",
        },
        PayloadFS: payload,
    }

    engine.Run(manifest, []engine.WizardPage{
        pages.Welcome(),
        pages.InstallLocation(),
        pages.ServiceAccount(),
        pages.Review(),
        pages.Install(manifest),
        pages.Complete(),
    })
}
```

## Build

```bash
cd payload && go build && cd ..
go build -ldflags="-H windowsgui" ./cmd/installer/
```

The resulting EXE is a complete installer. Run it to see the setup wizard. After installation, the same binary serves as the Windows service manager.

## Adding Pages

Add a license page:

```go
pages.License("MIT License\n\nCopyright (c) 2024 ...")
```

Add a settings form using the declarative Simple page:

```go
&pages.Simple{
    PageTitle: "Application Settings",
    Fields: []pages.Field{
        {Key: "port", Label: "HTTP Port", Type: "UNSIGNED", Default: "8080"},
        {Key: "verbose", Label: "Verbose logging", Type: "BOOL", Default: "false"},
    },
}
```

Add automatic updates:

```go
manifest.UpdateURL = "https://downloads.example.com/myapp-setup.exe"
// Add the AutoUpdates page to the wizard
pages.AutoUpdates()
```

## Next Steps

- [App Manifest](manifest.md) -- all manifest fields and derived identifiers
- [Wizard Pages](wizard-pages.md) -- full catalog of built-in pages
- [Custom Pages](custom-pages.md) -- implement the `WizardPage` interface for custom UI
- [Auto-Updates](auto-updates.md) -- how the update mechanism works

# ThePipes

ThePipes is a Go framework for building self-contained Windows installers and service managers. You compile a single EXE that contains your application payload, a setup wizard, and a Windows service wrapper. The same binary handles installation, service management, automatic updates, and uninstallation.

## Features

- **Single-EXE distribution** -- payload, wizard, and service manager in one file
- **Tk-based setup wizard** with built-in pages and support for custom pages
- **Windows service** with automatic restart, exponential backoff, and graceful shutdown
- **Desktop and Start Menu shortcuts** (URL and file targets)
- **Windows Firewall** rule management
- **Windows Event Log** integration -- severity-prefixed stdout lines from your app are forwarded to the Event Log
- **Rotating log files** (10 MB, one backup)
- **Automatic updates** via conditional HTTP (ETag/Last-Modified) with SHA-256 verification
- **Add/Remove Programs** registration with one-click uninstall
- **Variable expansion** (`%name%`, `%install%`, etc.) flowing from wizard inputs to service environment variables
- **Clean uninstall** with optional data preservation

## Choose Your Path

There are two ways to use ThePipes:

### Go Library

Import the module, define an `AppManifest`, compose wizard pages in Go, embed your payload with `//go:embed`, and compile a single EXE. Use this when you want full control, highly custom wizard pages, or if you just like working in Go.

Get started: [Getting Started with Go](getting-started-go.md)

### Endcap (JSON based)

Prepare a directory with `AppManifest.json`, `WizardPages.json`, and your application files. Run `endcap.exe --pack <dir>` to produce a self-contained installer. Custom logic uses [Tengo](https://github.com/d5/tengo) scripts. Use this if you don't want to bother with a Go compiler, or when the built-in pages cover all your needs.

Get started: [Getting Started with Endcap](getting-started-endcap.md)

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started with Go](getting-started-go.md) | Quickstart for Go library users |
| [Getting Started with Endcap](getting-started-endcap.md) | Quickstart for Endcap (no-code) users |
| [App Manifest](manifest.md) | `AppManifest` field reference and derived identifiers |
| [Variables](variables.md) | `%var%` expansion system and environment variable injection |
| [Wizard Pages](wizard-pages.md) | Built-in page catalog with Go and JSON examples |
| [Custom Pages](custom-pages.md) | Writing custom `WizardPage` implementations (Go only) |
| [Service Management](service-management.md) | Service lifecycle, logging, Event Log integration |
| [Auto-Updates](auto-updates.md) | Automatic update mechanism |
| [Install & Uninstall](install-uninstall.md) | Step-by-step install/uninstall sequences and binary modes |
| [Endcap Scripting](endcap-scripting.md) | Tengo scripting for Endcap Process and validation functions |

## License

This project is licensed under the [Mozilla Public License 2.0](../LICENSE). You can freely use ThePipes in proprietary software -- the copyleft applies only to modifications of ThePipes source files themselves.

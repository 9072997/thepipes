# ThePipes

ThePipes is a framework for building self-contained Windows installers and service managers. It targets environments like K-12 school districts where applications are installed by hand on Windows Server by IT administrators who need things to just work.

ThePipes produces a single executable that functions as a setup wizard, Windows service manager, and uninstaller. Application developers provide a minimal manifest, a payload of application files, and a sequence of wizard pages. The framework can be used as a Go library or via Endcap, a no-code JSON-driven carrier binary. The framework handles everything else.

## Design Philosophy

- **Single-file distribution.** The end user downloads one `.exe` and runs it. There is no MSI, no multi-step process, no external dependencies.
- **The setup wizard is the installer.** Configuration and installation happen in one continuous flow. The user is never told to "open a browser and navigate to localhost:8443 to continue setup." They fill out a wizard and click Next.
- **Applications stay simple.** Service binaries are ordinary Go programs that read configuration from environment variables and write logs to stdout. They do not import Windows service libraries, manage log rotation, or write to the Windows Event Log. The framework does all of that.
- **Clean boundaries.** The framework binary is separate from the application binary. The framework manages the service lifecycle; the application does its job.

## Application Manifest

Each application is described by a manifest:

```go
type AppManifest struct {
    Name       string            // "My Cool App" - used everywhere
    Publisher  string            // "Your Company"
    Version    string            // "1.2.3"
    Command    string            // "myapp.exe --config \"%data%\\config.yaml\""
    Shortcuts  map[string]string // shortcut name -> target path or URL
    UpdateURL  string            // "https://example.com/myapp-setup.exe"
    PayloadFS  fs.FS             // typically embed.FS via go:embed
}
```

The `Name` field drives all derived identifiers:

| Derived Value | Example |
|---|---|
| Install directory | `C:\Program Files\My Cool App\` |
| Data directory | `C:\ProgramData\My Cool App\` |
| Windows service name | Slugified from Name |
| Add/Remove Programs entry | Name, Publisher, Version |
| Event Log source | Name |
| Firewall rule | Executable-based, named after the app |

## Variable Substitution

The framework defines built-in variables and passes through all variables produced by wizard pages. Variables use `%name%` syntax and are expanded in the `Command` field, shortcut targets, and anywhere else strings appear in the manifest.

Built-in variables:

| Variable | Value |
|---|---|
| `%name%` | The application name, e.g. `My Cool App` |
| `%publisher%` | The publisher, e.g. `Your Company` |
| `%version%` | The version string, e.g. `1.2.3` |
| `%command%` | The raw command string from the manifest |
| `%install%` | The install directory, e.g. `C:\Program Files\My Cool App` |
| `%data%` | The data directory, e.g. `C:\ProgramData\My Cool App` |
| `%update_url%` | The update URL (only present when UpdateURL is set) |

All other variables come from wizard page results. For example, if an ACME wizard page produces an `fqdn` entry, it is available as `%fqdn%` everywhere.

### Example Usage

```go
Command: `myapp.exe --config "%data%\app-config.yaml" --listen :%port%`,
Shortcuts: map[string]string{
    "My Cool App":   "https://localhost:%port%",
    "Configuration": `%data%\app-config.yaml`,
    "README":        `%install%\README.txt`,
    "Logs":          `%data%\logs\service.log`,
    "Documentation": "https://docs.example.com",
},
```

Shortcut targets are interpreted by their content. URLs starting with `http://` or `https://` become `.url` internet shortcut files. Everything else becomes a `.lnk` file pointing to the resolved path. All shortcuts are placed on the desktop and in the Start Menu.

## Configuration Storage

The accumulated results of all wizard pages are written to `%data%\install-config.json` as a flat JSON object of string key-value pairs. Keys starting with `.` (e.g. `.service_password`) are considered hidden - they are excluded from `install-config.json` and from the Review page display. Use the `.` prefix for sensitive values.

At service start, the framework reads this file, merges it with the built-in variables, expands all `%variable%` references in `Command`, and launches the application process with every key-value pair injected as an uppercase environment variable (e.g. `port` becomes `PORT`).

The application reads its configuration from environment variables. It does not need to know about `install-config.json`, the Windows registry, or any framework internals.

## Wizard Pages

The setup wizard is built with [modernc.org/tk9.0](https://pkg.go.dev/modernc.org/tk9.0), a CGo-free pure Go port of Tcl/Tk 9.0. It produces a native windowed application with no runtime dependencies on the target machine.

Each wizard page implements a common interface:

```go
type WizardPage interface {
    Title() string
    Skip(vars map[string]string) bool
    Render(parent *tk.FrameWidget, vars map[string]string, nav NavControl)
    Validate() error
    Results() map[string]string
}
```

The `NavControl` struct gives pages the ability to control wizard navigation:

```go
type NavControl struct {
    Advance        func()             // programmatically click Next
    DisableReentry func()             // prevent Back navigation to this page
    SetBack        func(enabled bool) // enable/disable Back button
    SetNext        func(enabled bool) // enable/disable Next button
}
```

Pages are composed into a sequence when defining the installer:

```go
engine.Run(manifest, []engine.WizardPage{
    pages.Welcome(),
    pages.License("License text..."),
    pages.InstallLocation(),
    pages.ServiceAccount(),
    acme.NewPage(),
    myapp.OAuthConfigPage(),
    pages.AutoUpdates(),
    pages.Review(),
    pages.Install(manifest),
    pages.Complete(),
})
```

### Shared Modules

The framework ships with reusable wizard page modules for common tasks:

- **Welcome** - branding, version, publisher information.
- **License** - scrollable license text with an "I accept" checkbox gating advancement.
- **Simple** - declarative form builder. Define fields with a key, label, type (TEXT, UNSIGNED, INT, FLOAT, FILE, FOLDER, BOOL, or slash-separated dropdown options like `"a/b/c"`), and default value. Optional custom validation function.
- **Process** - runs a function in the background with a scrolling log and progress bar. Lines containing a percentage (e.g. `47%`) drive the progress bar. Auto-advances when the function returns.
- **InstallLocation** - choose the install directory with a Browse button. Validates the directory is writable.
- **ServiceAccount** - configure the service to run as Local Service or a specific domain account.
- **AutoUpdates** - checkbox to enable or disable automatic update checking. Skipped automatically when `UpdateURL` is not set.
- **ACME / Let's Encrypt** - detect public IP addresses, test certificate issuance via Let's Encrypt staging, and let the user select an address or enter a custom FQDN.
- **Review** - display a summary of all configuration before committing. Hidden keys (`.` prefix) are excluded.
- **Install** - progress bar during file extraction, service registration, and other install operations.
- **Complete** - confirmation that installation is complete and the service has been started.

### Page Communication

The `vars` parameter passed to `Render` and `Skip` contains the merged results from all previously completed wizard pages plus the built-in variables. For example, if the ACME page returns `{"fqdn": "app.school.org"}` from `Results()`, subsequent pages receive that value in their `vars` map and can use it to pre-populate fields or build instructions. Pages that don't need prior context simply ignore the parameter.

### Conditional Pages

The framework checks `Skip(vars)` before rendering each page. If it returns true, the page is never shown and the wizard advances to the next one. This keeps the page sequence declarative - every page that *might* appear is listed, and each page decides for itself whether it's relevant based on what previous pages have configured. For example, the AutoUpdates page skips itself when no update URL is configured:

```go
func (p *autoUpdatesPage) Skip(vars map[string]string) bool {
    return vars["update_url"] == ""
}
```

### Custom Pages

Application-specific wizard pages are ordinary Go code. They implement the same `WizardPage` interface and have full access to the Tk widget set for building their UI and to Go's standard library for any backend logic. Pages that need to perform long-running operations (like polling DNS or waiting for an OAuth callback) manage their own async work internally using goroutines and Tk UI updates. The `Validate` method gates advancement - the framework only moves to the next page when `Validate` returns nil.

## Service Management

When the framework binary is registered as a Windows service, it runs in service mode and manages the application as a child process.

### Process Lifecycle

The framework starts the application process with expanded command-line arguments, the full set of configuration variables as environment variables, and stdout/stderr captured to a log file in `%data%\logs\`. If the application process exits unexpectedly, the framework restarts it with backoff. On service stop, the framework signals the child process and waits for a grace period before terminating it.

### Log Rotation

The framework captures all stdout and stderr output to `%data%\logs\service.log`. When the log file reaches 10 MB, the framework rotates it: the current file is renamed to `service.old.log` (replacing any existing old log), and a new `service.log` is started. One current and one old log file are kept.

### Windows Event Log Integration

The framework automatically parses log lines from the application's stdout and stderr. Lines beginning with a recognized prefix are stripped of that prefix and forwarded to the Windows Event Log under the application's name with the corresponding severity level:

| Prefix | Event Log Level |
|---|---|
| `ERROR:` | Error |
| `WARNING:` | Warning |
| `INFO:` | Information |

Lines without a recognized prefix are written only to the log file.

From the application developer's perspective, logging to the Windows Event Log is as simple as writing to stdout:

```go
fmt.Println("INFO: server started on port 8443")
fmt.Println("WARNING: certificate expires in 7 days")
fmt.Println("ERROR: failed to connect to database")
fmt.Println("this line just goes to the log file")
```

The framework registers the application as an Event Log source on install and removes it on uninstall.

## Binary Modes

The framework binary serves multiple roles depending on how it is invoked:

| Invocation | Behavior |
|---|---|
| `myapp-setup.exe` | Launch the setup wizard GUI |
| `myapp-setup.exe --service` | Run as a Windows service (invoked by the Service Control Manager) |
| `myapp-setup.exe --update` | Apply a downloaded update and restart the service |
| `myapp-setup.exe --apply-update <dir>` | Extract payload to a directory (no service operations) |
| `myapp-setup.exe --uninstall` | Uninstall the application |
| `myapp-setup.exe --cleanup <dir>` | Post-uninstall file deletion (internal use) |

## Auto-Update

ThePipes includes a zero-infrastructure auto-update mechanism. The only requirement is a web server that hosts the current version of the installer executable.

### How It Works

The `UpdateURL` field in the manifest points to a copy of the installer `.exe` hosted on any static web server. The framework periodically checks this URL for changes and applies updates automatically.

**On install,** the framework computes a SHA-256 hash of its own executable and stores it in the registry at `HKLM\SOFTWARE\{Name}\Update` along with an empty ETag and Last-Modified value.

**Periodically while the service is running,** the framework makes an HTTP HEAD (or conditional GET) request to the `UpdateURL`, sending the stored `ETag` and `Last-Modified` values as `If-None-Match` and `If-Modified-Since` headers. If the server responds with `304 Not Modified`, no work is done. If the server responds with new content, the framework downloads the file to a temporary location and computes its SHA-256 hash. If the hash matches the currently running executable, the new `ETag` and `Last-Modified` values are stored and no further action is taken. If the hash differs, an update is applied.

**The update flow:**

1. Download the new installer `.exe` to a temporary location.
2. Verify the download is a valid PE executable.
3. Stop the application child process.
4. Replace the contents of the install directory by running the new executable with a `--apply-update` flag, which extracts the embedded payload without showing the wizard.
5. Replace the service manager binary itself (using the same temp-copy-and-swap technique used by uninstall).
6. Update the stored hash, ETag, and Last-Modified values in the registry.
7. Restart the service.

The existing `install-config.json` in the data directory is left untouched. The wizard is never shown. The application restarts with the same configuration it had before.

### Registry Storage

Update state is stored at `HKLM\SOFTWARE\{Name}\Update`:

| Value | Type | Purpose |
|---|---|---|
| `CurrentHash` | REG_SZ | SHA-256 of the currently installed executable |
| `ETag` | REG_SZ | Last ETag received from the update server |
| `LastModified` | REG_SZ | Last Last-Modified value received from the update server |

This registry key is removed during uninstall.

### Server Requirements

The update server is any web server capable of serving a static file. There is no update API, no version manifest, and no delta patching. The framework relies entirely on standard HTTP caching headers to avoid unnecessary downloads. Simply replace the `.exe` on the server to push an update.

## Install Sequence

When the user runs the setup wizard and completes all configuration pages, the framework performs the following steps:

1. Extract the embedded payload filesystem to `C:\Program Files\{Name}\`.
2. Copy the installer binary to the install directory as the persistent service manager (`{slug}-setup.exe`).
3. Create the data directory at `C:\ProgramData\{Name}\`.
4. Write `install-config.json` to the data directory with all wizard results (excluding hidden `.` keys).
5. Register the framework binary as a Windows service in `--service` mode.
6. Add an executable-based Windows Firewall rule for the application binary.
7. Create desktop and Start Menu shortcuts.
8. Register the application as a Windows Event Log source.
9. Write the Add/Remove Programs registry key under `HKLM\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\{Name}`, pointing the uninstall command to the framework binary with `--uninstall`.
10. Initialize update state - if `UpdateURL` is set, compute the SHA-256 hash of the installed binary and write it to the registry.
11. Start the service.

## Uninstall Sequence

When uninstall is triggered (from Add/Remove Programs or by running the binary with `--uninstall`):

1. Stop the Windows service.
2. Deregister the service.
3. Remove the Windows Firewall rule.
4. Remove desktop and Start Menu shortcuts.
5. Remove the Add/Remove Programs registry key.
6. Remove the Event Log source registration.
7. Remove the update state registry key.
8. Optionally ask whether to keep or delete the data directory.
9. Copy the framework binary to the system temp directory with a random filename.
10. Launch the temp copy with `--cleanup` pointing to the install directory, then exit.
11. The cleanup copy waits for the original process to exit, deletes the install directory (retrying on locked files), and marks itself for deletion on reboot via `MoveFileEx`.

## Bundling External Dependencies

For applications that require external tools like PostgreSQL, a JDK, or Tomcat, these should be placed in the payload directory using their portable/zip distributions (not their system installers). They are embedded into the installer binary alongside the application and extracted into the install directory. This keeps the install self-contained and avoids conflicts with other software on the system. Uninstall removes everything cleanly.

## Building an Installer

A typical project using ThePipes has a `cmd/installer` directory:

```go
package main

import (
    "embed"

    "github.com/9072997/thepipes/engine"
    "github.com/9072997/thepipes/pages"
    "github.com/9072997/thepipes/pages/acme"
    "myapp/installer/custom"
)

//go:embed payload
var payload embed.FS

func main() {
    manifest := engine.AppManifest{
        Name:      "My Cool App",
        Publisher: "Your Company",
        Version:   "1.2.3",
        Command:   `myapp.exe --config "%data%\app-config.yaml" --listen :%port%`,
        Shortcuts: map[string]string{
            "My Cool App":   "https://localhost:%port%",
            "Configuration": `%data%\app-config.yaml`,
            "Logs":          `%data%\logs\service.log`,
        },
        UpdateURL: "https://downloads.example.com/myapp-setup.exe",
        PayloadFS: payload,
    }

    engine.Run(manifest, []engine.WizardPage{
        pages.Welcome(),
        pages.InstallLocation(),
        acme.NewPage(),
        custom.MyAppSpecificPage(),
        pages.AutoUpdates(),
        pages.Review(),
        pages.Install(manifest),
        pages.Complete(),
    })
}
```

The build pipeline compiles the application binary, places it (along with any bundled dependencies) into the `payload` directory, and then compiles the installer binary. The `go:embed` directive embeds the entire directory tree. The result is a single `.exe` that the user downloads and runs.

## Endcap (No-Code Installer)

For teams that don't want to write Go, ThePipes provides **Endcap** - a pre-compiled carrier binary that reads its manifest and page sequence from JSON files. Custom logic is written in [Tengo](https://github.com/d5/tengo) scripts instead of Go.

### Payload Directory

```
my-payload/
  AppManifest.json       # same fields as the Go struct (minus PayloadFS)
  WizardPages.json       # array of page specs
  myapp.exe              # application binary
  LICENSE.txt            # optional, referenced by License page
  scripts/setup.tengo    # optional, referenced by Process page
```

### WizardPages.json

Each element is either a bare string (page type name with defaults) or an object with a `Type` field plus extra settings:

```json
[
  "Welcome",
  {"Type": "License", "File": "LICENSE.txt"},
  {"Type": "Simple", "PageTitle": "Settings", "Fields": [...]},
  {"Type": "Process", "PageTitle": "Setup", "Func": "scripts/setup.tengo"},
  "InstallLocation",
  "ServiceAccount",
  "AutoUpdates",
  "Review",
  "Install",
  "Complete"
]
```

### Packing

Running `endcap.exe --pack my-payload/` validates the JSON files, creates a ZIP from the directory, appends it to the carrier binary, and outputs a self-contained installer like `my-app-1.2.3-setup.exe`.

See [Getting Started with Endcap](docs/getting-started-endcap.md) and [Endcap Scripting](docs/endcap-scripting.md) for details.

# Wizard Pages

The setup wizard is composed of pages. Each page collects input, performs work, or displays information. Pages are shown in order, skipping any whose `Skip()` method returns true.

## Page Lifecycle

Each page goes through these steps:

1. **Skip** -- `Skip(vars)` is called. If it returns true, the page is hidden and the wizard advances.
2. **Render** -- `Render(parent, vars, nav)` builds the page UI.
3. **Validate** -- When the user clicks Next, `Validate()` is called. If it returns an error, a dialog is shown and the page stays.
4. **Results** -- On successful validation, `Results()` returns key-value pairs that are merged into the shared vars map.

## NavControl

Pages receive a `NavControl` struct to influence navigation:

| Method | Description |
|--------|-------------|
| `Advance()` | Programmatically click Next (e.g., after an async task completes) |
| `DisableReentry()` | Prevent the user from navigating back to this page |
| `SetBack(enabled)` | Enable or disable the Back button |
| `SetNext(enabled)` | Enable or disable the Next button |

## Built-in Pages

### Welcome

Displays the application name, version, publisher, and default directories. No configuration, no results.

**Go:** `pages.Welcome()`

**Endcap:** `"Welcome"`

### License

Displays scrollable license text with an "I accept" checkbox. The Next button is disabled until the checkbox is checked. No results.

**Go:** `pages.License("License text here...")`

**Endcap:**
```json
{
  "Type": "License",
  "File": "LICENSE.txt"
}
```

The `File` field is a path to a text file in the payload directory.

### Simple

A declarative form builder that generates labeled input fields from a list of field specs.

**Go:**
```go
&pages.Simple{
    PageTitle: "Application Settings",
    Fields: []pages.Field{
        {Key: "port", Label: "HTTP Port", Type: "UNSIGNED", Default: "8080"},
        {Key: "message", Label: "Welcome message", Type: "TEXT", Default: "Hello"},
        {Key: "data_dir", Label: "Data folder", Type: "FOLDER", Default: ""},
        {Key: "cert", Label: "Certificate file", Type: "FILE", Default: ""},
        {Key: "verbose", Label: "Verbose logging", Type: "BOOL", Default: "false"},
        {Key: "mode", Label: "Run mode", Type: "development/staging/production", Default: "production"},
    },
    ValidateFunc: func(vars map[string]string) error {
        // Optional custom validation after built-in type checks.
        return nil
    },
}
```

**Endcap:**
```json
{
  "Type": "Simple",
  "PageTitle": "Application Settings",
  "Fields": [
    {"Key": "port", "Label": "HTTP Port", "Type": "UNSIGNED", "Default": "8080"},
    {"Key": "verbose", "Label": "Verbose logging", "Type": "BOOL", "Default": "false"}
  ],
  "ValidateFunc": "scripts/validate-settings.tengo"
}
```

#### Field Types

| Type | Widget | Validation |
|------|--------|------------|
| `TEXT` | Text input | None |
| `UNSIGNED` | Text input | Must be a non-negative integer |
| `INT` | Text input | Must be an integer |
| `FLOAT` | Text input | Must be a number |
| `FILE` | Text input + Browse button | None (file picker dialog) |
| `FOLDER` | Text input + Browse button | None (folder picker dialog) |
| `BOOL` | Checkbox | Always valid. Stored as `"true"` or `"false"` |
| `a/b/c` | Dropdown (combobox) | Restricted to listed values |

Results: one entry per field, keyed by `Field.Key`.

### Process

Runs a function in the background and displays its output in a scrolling log with a progress bar.

**Go:**
```go
&pages.Process{
    PageTitle: "Preparing Database",
    Func: func(log io.Writer, vars map[string]string) {
        fmt.Fprintln(log, "Creating tables...")
        // ... do work ...
        fmt.Fprintln(log, "Progress: 50%")
        // ... more work ...
        fmt.Fprintln(log, "Done: 100%")
    },
    Reentry: false,
}
```

**Endcap:**
```json
{
  "Type": "Process",
  "PageTitle": "Preparing Database",
  "Func": "scripts/setup-db.tengo",
  "Reentry": false
}
```

#### Behavior

- Lines written to `log` appear in the scrolling text widget.
- If a line contains a percentage (e.g., `47%` or `12.5%`), the progress bar switches from indeterminate (spinner) to determinate mode and updates to that value.
- The page auto-advances when the function returns.
- Back and Next buttons are disabled while the function runs.
- `Reentry: false` (the default) prevents navigating back to this page after completion.

### InstallLocation

Lets the user change the install directory with a text field and Browse button. Validates that the directory is writable.

**Go:** `pages.InstallLocation()`

**Endcap:** `"InstallLocation"`

**Results:** `install` (overrides the built-in default)

### ServiceAccount

Lets the user choose between Local Service and a domain/custom account. Domain accounts require a username and password.

**Go:** `pages.ServiceAccount()`

**Endcap:** `"ServiceAccount"`

**Results:** `service_account` (e.g., `"LocalService"` or `"DOMAIN\user"`), `.service_password`

### AutoUpdates

Checkbox to enable automatic update checking. **Skipped automatically** when `update_url` is not set in the manifest.

**Go:** `pages.AutoUpdates()`

**Endcap:** `"AutoUpdates"`

**Results:** `auto_updates` (`"true"` or `"false"`)

### Review

Displays all accumulated variables in a two-column table. Keys starting with `.` are hidden. No results.

**Go:** `pages.Review()`

**Endcap:** `"Review"`

### Install

Runs the installation sequence (see [Install & Uninstall](install-uninstall.md)) as a Process page with a progress log.

**Go:** `pages.Install(manifest)`

**Endcap:** `"Install"`

This page must be present -- the Endcap packer will reject configurations without it.

### Complete

Displays a "Setup Complete" message. The Next button reads "Finish" and closes the wizard.

**Go:** `pages.Complete()`

**Endcap:** `"Complete"`

### ACME (Let's Encrypt)

Detects the machine's public IPv4 and IPv6 addresses, then tests certificate issuance against the Let's Encrypt staging environment. The user selects an IPv4 address, IPv6 address, or enters a custom FQDN.

**Go:** `acme.NewPage()` (import `github.com/9072997/thepipes/pages/acme`)

**Endcap:** Not available.

**Results:** `fqdn`

## Recommended Page Order

A typical installer for a web service:

```go
pages.Welcome(),
pages.License(licenseText),
// ... custom settings pages ...
pages.InstallLocation(),
pages.ServiceAccount(),
pages.AutoUpdates(),
pages.Review(),
pages.Install(manifest),
pages.Complete(),
```

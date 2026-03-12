# App Manifest

The manifest describes your application. In Go it is the `engine.AppManifest` struct; in Endcap it is `AppManifest.json`.

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | string | yes | Display name shown in the wizard, service, shortcuts, and Add/Remove Programs. |
| `Publisher` | string | no | Company or author name shown on the Welcome page and in Add/Remove Programs. |
| `Version` | string | no | Version string shown on the Welcome page and in Add/Remove Programs. |
| `Command` | string | yes | Command line to run as a service. Supports `%var%` expansion. Example: `myapp.exe --port %port%` |
| `Shortcuts` | map[string]string | no | Shortcut name to target. URL targets (starting with `http://` or `https://`) create `.url` files. File paths create `.lnk` files via COM. Targets support `%var%` expansion. |
| `UpdateURL` | string | no | URL where the current installer EXE is hosted. Enables automatic updates when set. |
| `PayloadFS` | fs.FS | Go only | Embedded filesystem containing your application files. Typically an `embed.FS` via `//go:embed`. Endcap uses the ZIP contents automatically. |

## Derived Identifiers

ThePipes derives several identifiers from `Name`. For example, if `Name` is `"My Cool App"`:

| Identifier | Value | Used For |
|------------|-------|----------|
| Slug | `my-cool-app` | Service name, binary filename |
| Binary name | `my-cool-app-setup.exe` | Installer/service binary in install dir |
| Install directory | `C:\Program Files\My Cool App` | Payload extraction target |
| Data directory | `C:\ProgramData\My Cool App` | Runtime config, logs |
| Service name | `my-cool-app` | Windows service registration |
| ARP entry | `My Cool App` | Add/Remove Programs display name |
| Event Log source | `My Cool App` | Windows Event Log source name |
| Firewall rule | `My Cool App` | Windows Firewall rule name |

The slug is computed by lowercasing the name, replacing non-alphanumeric characters with hyphens, and collapsing consecutive hyphens.

## Shortcuts

Shortcuts are created in two locations: the public Desktop (`C:\Users\Public\Desktop`) and the Start Menu (`C:\ProgramData\Microsoft\Windows\Start Menu\Programs`).

- **URL targets** (starting with `http://` or `https://`) become `.url` files (INI format).
- **File targets** become `.lnk` files created via COM (`WScript.Shell`).

Targets support `%var%` expansion. For example:

```go
Shortcuts: map[string]string{
    "My App":         "http://localhost%listen%",       // .url shortcut
    "Install Config": `%data%\install-config.json`,     // .lnk shortcut
}
```

## PayloadFS and File Matching

The payload filesystem is wrapped by an internal `cleanfs` layer that provides:

- **Case-insensitive file lookups** -- files are matched regardless of case.
- **Automatic prefix stripping** -- if the embedded FS has a single nested directory (e.g., `payload/`), it is transparently skipped so files appear at the root.

## Window Icon

If the payload contains a file named `installer.{ext}` at the root, it is used as the wizard window icon. Supported formats (checked in order): `png`, `ico`, `gif`, `jpg`, `jpeg`, `bmp`, `svg`, `tiff`, `tif`.

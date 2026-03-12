# Variables

ThePipes uses a `%key%` token syntax for variable expansion. Variables flow from the manifest and wizard pages through to the service runtime.

## Syntax

Variables are written as `%key%` and expanded by `engine.Expand()`. For example, `%data%\logs` might expand to `C:\ProgramData\My App\logs`.

## Built-in Variables

These are always available, derived from the manifest:

| Variable | Example Value | Source |
|----------|---------------|--------|
| `%name%` | `My App` | `AppManifest.Name` |
| `%publisher%` | `My Company` | `AppManifest.Publisher` |
| `%version%` | `1.0.0` | `AppManifest.Version` |
| `%command%` | `myapp.exe --port %port%` | `AppManifest.Command` |
| `%install%` | `C:\Program Files\My App` | Derived from Name |
| `%data%` | `C:\ProgramData\My App` | Derived from Name |
| `%update_url%` | `https://...` | `AppManifest.UpdateURL` (only if set) |

## Page-Produced Variables

Each wizard page's `Results()` method adds variables to the shared map:

| Page | Variables Produced |
|------|--------------------|
| Welcome | (none) |
| License | (none) |
| Simple | One per field, keyed by `Field.Key` |
| Process | (none, but the function can mutate `vars` in Endcap) |
| InstallLocation | `install` |
| ServiceAccount | `service_account`, `.service_password` |
| AutoUpdates | `auto_updates` (`"true"` or `"false"`) |
| Review | (none) |
| Install | (none) |
| Complete | (none) |
| ACME | `fqdn` |

## Where Expansion Happens

- **Command** -- expanded at service start when launching the child process
- **Shortcut targets** -- expanded at install time
- **Anywhere you call `engine.Expand(s, vars)`** in custom pages or Go code

## Configuration Storage

After installation, all wizard variables (except built-in ones) are written to `%data%\install-config.json`. At service start:

1. The service reads `install-config.json`
2. Merges with built-in variables (built-ins take precedence for `install`, `data`, etc.)
3. Expands the `Command` field
4. Injects all variables as **uppercase environment variables** for the child process

For example, a variable `port` with value `8080` becomes the environment variable `PORT=8080`.

## Hidden Keys

Keys starting with `.` (like `.service_password`) are:
- Excluded from the Review page display
- Excluded from `install-config.json`

Use the `.` prefix for sensitive values that should not be persisted or shown to the user after entry.

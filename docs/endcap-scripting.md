# Endcap Scripting

Endcap uses [Tengo](https://github.com/d5/tengo) for custom logic in Process pages and Simple page validation. Tengo is a Go-embedded scripting language with a small, safe standard library.

## Process Scripts

A Process page `Func` points to a `.tengo` file in the payload directory. The script receives two globals:

### `log`

A writer object with two methods:

- `log.println(args...)` -- prints arguments separated by spaces, with a trailing newline.
- `log.printf(format, args...)` -- prints a formatted string with a trailing newline (appended if not present).

Lines written via `log` appear in the scrolling log widget. If a line contains a percentage (e.g., `47%`), the progress bar updates.

### `vars`

A map of all wizard variables collected so far. You can read existing values, add new keys, update values, and delete keys. All mutations are synced back to the wizard's variable map when the script finishes.

### Example

```tengo
fmt := import("fmt")
os := import("os")

log.println("Setting up database...")

port := vars["db_port"]
if port == "" {
    port = "5432"
}

log.printf("Using port %s", port)
log.println("Creating tables... 25%")

// Simulate work
times := import("times")
times.sleep(2 * times.second)

log.println("Inserting seed data... 75%")
times.sleep(1 * times.second)

// Set a new variable that later pages can use
vars["db_initialized"] = "true"

log.println("Database ready. 100%")
```

Reference in `WizardPages.json`:
```json
{
  "Type": "Process",
  "PageTitle": "Preparing Database",
  "Func": "scripts/setup-db.tengo"
}
```

## Validation Scripts

A Simple page `ValidateFunc` points to a `.tengo` file. The script receives:

### `vars`

Same as Process scripts -- a mutable map of current wizard variables (including the fields from the current Simple page). Mutations are synced back.

### `error`

Initially an empty string. Set it to a non-empty string to fail validation. The string becomes the error message shown to the user.

### Example

```tengo
port := vars["port"]

fmt := import("fmt")
text := import("text")

n := text.atoi(port)
if is_error(n) {
    error = "Port must be a number"
} else if n < 1 || n > 65535 {
    error = fmt.sprintf("Port %d is out of range (1-65535)", n)
} else if n < 1024 {
    error = fmt.sprintf("Port %d requires administrator privileges; use 1024 or higher", n)
}
```

Reference in `WizardPages.json`:
```json
{
  "Type": "Simple",
  "PageTitle": "Network Settings",
  "Fields": [
    {"Key": "port", "Label": "HTTP Port", "Type": "UNSIGNED", "Default": "8080"}
  ],
  "ValidateFunc": "scripts/validate-port.tengo"
}
```

## Available Standard Library

All Tengo standard library modules are available:

| Module | Purpose |
|--------|---------|
| `fmt` | String formatting |
| `os` | OS functions (file I/O, env vars) |
| `text` | String manipulation, conversion |
| `math` | Math functions |
| `times` | Time, sleep, duration |
| `rand` | Random numbers |
| `json` | JSON encode/decode |
| `base64` | Base64 encode/decode |
| `hex` | Hex encode/decode |
| `enum` | Enumeration helpers |

Import modules with `import("module_name")`.

## File References

Script paths in `WizardPages.json` are relative to the payload directory root:

```
my-payload/
  WizardPages.json
  scripts/
    setup-db.tengo        # "Func": "scripts/setup-db.tengo"
    validate-port.tengo   # "ValidateFunc": "scripts/validate-port.tengo"
```

The Endcap packer validates that all referenced script files exist at pack time.

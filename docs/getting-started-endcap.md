# Getting Started with Endcap

Endcap is a pre-compiled carrier binary that reads its configuration from JSON files. You don't need Go or any build tools -- just prepare a payload directory and pack it.

## Payload Directory Layout

```
my-payload/
  AppManifest.json       # required -- describes your application
  WizardPages.json       # required -- defines the setup wizard flow
  myapp.exe              # your application binary
  LICENSE.txt            # optional -- referenced by License page
  scripts/
    setup.tengo          # optional -- Tengo script for Process page
```

## AppManifest.json

```json
{
  "Name": "My App",
  "Publisher": "My Company",
  "Version": "1.0.0",
  "Command": "myapp.exe --config \"%data%\\config.toml\"",
  "Shortcuts": {
    "My App": "http://localhost:8080"
  },
  "UpdateURL": "https://downloads.example.com/my-app-setup.exe"
}
```

See [App Manifest](manifest.md) for all fields.

## WizardPages.json

Pages can be bare strings (for pages that don't require configuration) or objects with settings. See [Wizard Pages](wizard-pages.md) for all options.

```json
[
  "Welcome",
  {
    "Type": "License",
    "File": "LICENSE.txt"
  },
  {
    "Type": "Simple",
    "PageTitle": "Application Settings",
    "Fields": [
      {"Key": "port", "Label": "HTTP Port", "Type": "UNSIGNED", "Default": "8080"},
      {"Key": "verbose", "Label": "Verbose logging", "Type": "BOOL", "Default": "false"}
    ]
  },
  "InstallLocation",
  "ServiceAccount",
  "AutoUpdates",
  "Review",
  "Install",
  "Complete"
]
```

## Packing

```bash
endcap.exe --pack my-payload/
```

This validates your JSON files, creates a ZIP from the directory, appends it to the carrier binary, and outputs a file like `my-app-1.0.0-setup.exe` in the current directory.

### Validation

The pack command checks:
- `AppManifest.json` exists and has `Name` and `Command`
- `WizardPages.json` exists, is valid JSON, and contains an `Install` page
- All referenced files exist (License `File`, Process `Func`, Simple `ValidateFunc`)
- All page types are recognized

## Custom Logic with Tengo

For Process pages that run tasks, or Simple pages that need custom validation, you can write [Tengo scripts](endcap-scripting.md):

```json
{
  "Type": "Process",
  "PageTitle": "Preparing Database",
  "Func": "scripts/setup.tengo"
}
```

## Next Steps

- [App Manifest](manifest.md) -- all manifest fields
- [Wizard Pages](wizard-pages.md) -- full page catalog
- [Endcap Scripting](endcap-scripting.md) -- Tengo script reference
- [Variables](variables.md) -- the `%var%` expansion system

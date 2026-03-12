# Auto-Updates

ThePipes includes a built-in auto-update mechanism. The service periodically checks a URL for a new version of the installer and applies updates without user interaction.

## Requirements

- A web server hosting the current installer EXE at a stable URL
- `UpdateURL` set in the manifest
- The AutoUpdates wizard page included (or omitted to always enable updates)

## Setup

**Go:**
```go
manifest := engine.AppManifest{
    // ...
    UpdateURL: "https://downloads.example.com/my-app-setup.exe",
}
// Include in wizard pages:
pages.AutoUpdates()
```

**Endcap (AppManifest.json):**
```json
{
  "UpdateURL": "https://downloads.example.com/my-app-setup.exe"
}
```

If the AutoUpdates page is included, the user can opt out. If omitted, updates are always enabled when `UpdateURL` is set.

## How It Works

### Check Cycle

1. The service runs an update check at startup, then every **1 hour**.
2. A **HEAD request** is sent to the `UpdateURL` with conditional headers:
   - `If-None-Match: {stored ETag}`
   - `If-Modified-Since: {stored Last-Modified}`

### Response Handling

- **304 Not Modified:** No new version on the server. However, if the installed binary's SHA-256 hash doesn't match the stored hash (e.g., it was modified externally), an update is triggered anyway.
- **200 OK:** The file may have changed. ThePipes downloads it to a temp file, computes the SHA-256 hash, and compares:
  - Same hash as the installed binary: no update needed; just refreshes the stored ETag and Last-Modified headers.
  - Different hash: validates the downloaded file is a PE executable (checks MZ magic bytes), then launches the update process.

### Update Process

1. The downloaded binary is launched with `--update` in a detached process.
2. The update process stops the service.
3. Extracts the new payload to the install directory (overwrites existing files).
4. Copies the new binary to the service manager path.
5. Updates the registry with the new SHA-256 hash.
6. Restarts the service.
7. Schedules the temp update file for deletion on reboot.

### What Gets Preserved

- `install-config.json` in the data directory is **never touched** during updates.
- The wizard is never shown. The existing configuration is kept.
- Only the payload files and the service binary are replaced.

## Registry State

Update state is stored at `HKLM\SOFTWARE\{Name}\Update`:

| Value | Purpose |
|-------|---------|
| `CurrentHash` | SHA-256 hex digest of the installed binary |
| `ETag` | Last ETag header from the server |
| `LastModified` | Last Last-Modified header from the server |

## Server Tips

Any static file server works. Recommended options:

- **Amazon S3** or **Google Cloud Storage** -- automatic ETag headers
- **nginx** or **IIS** -- returns ETag and Last-Modified by default for static files
- **GitHub Releases** -- works, but URLs change per release

Make sure your server returns `ETag` or `Last-Modified` headers. Without them, every check results in a full download comparison (still works, just less efficient).

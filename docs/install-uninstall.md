# Install & Uninstall

## Binary Modes

The installer binary auto-detects its mode from command-line arguments:

| Arguments | Mode | Description |
|-----------|------|-------------|
| *(none)* | Wizard | Shows the setup wizard (requests admin elevation) |
| `--service` | Service | Runs as a Windows service via SCM |
| `--update` | Update | Applies a downloaded update and restarts the service |
| `--apply-update <dir>` | Apply Update | Extracts payload to a directory (no service operations) |
| `--uninstall` | Uninstall | Runs the uninstall sequence with UI dialogs |
| `--cleanup <dir>` | Cleanup | Deferred removal of the install directory |

## Install Sequence

When the Install page runs, it performs these steps in order:

1. **Extract payload** -- Writes all files from the embedded filesystem to `%ProgramFiles%\{Name}\`.
2. **Copy installer binary** -- Copies itself to the install directory as `{slug}-setup.exe`. This binary serves as the service manager going forward.
3. **Create data directory** -- Creates `%ProgramData%\{Name}\` for runtime configuration and logs.
4. **Write configuration** -- Saves wizard variables to `install-config.json` in the data directory. Keys starting with `.` are excluded.
5. **Register Windows service** -- Creates a Windows service named after the app slug, set to auto-start. Uses the service account selected in the wizard (default: `NT AUTHORITY\LocalService`). Recovery policy: restart on first two failures with 5-second delays.
6. **Add firewall rule** -- Creates an inbound allow rule for the application executable via `netsh`. Non-fatal on failure.
7. **Create shortcuts** -- Places shortcuts on the public Desktop and Start Menu. URL targets become `.url` files; file targets become `.lnk` files.
8. **Register Event Log source** -- Registers the app name as a Windows Event Log source.
9. **Register in Add/Remove Programs** -- Writes an ARP entry with display name, version, publisher, install location, and uninstall command.
10. **Initialize update state** -- If `UpdateURL` is set, computes the SHA-256 hash of the installed binary and writes it to the registry.
11. **Start service** -- Starts the Windows service.

## Uninstall Sequence

Running the binary with `--uninstall` shows a confirmation dialog, then performs:

1. **Stop service** -- Sends stop signal and waits for the service to halt.
2. **Delete service** -- Removes the service registration from SCM.
3. **Remove firewall rule** -- Deletes the inbound allow rule.
4. **Remove shortcuts** -- Deletes desktop and Start Menu shortcuts.
5. **Remove ARP entry** -- Deletes the Add/Remove Programs registry key.
6. **Unregister Event Log source** -- Removes the Event Log source registration.
7. **Remove update state** -- Deletes update-related registry keys.
8. **Ask about data** -- Shows a dialog asking whether to keep or delete the data directory (`%ProgramData%\{Name}\`).
9. **Delete data directory** -- If the user chose to delete.
10. **Copy to temp** -- Copies itself to a temp file (since the binary can't delete itself while running).
11. **Launch cleanup** -- Starts the temp copy with `--cleanup <installDir>`, then exits.

### Cleanup Mode

The cleanup process (launched as step 11 above):

1. Waits up to 30 seconds for the install directory to become unlocked (probes via rename).
2. Attempts `RemoveAll` on the install directory, up to 5 retries with 1-second delays.
3. Schedules its own temp file for deletion on the next Windows reboot using `MoveFileEx` with `MOVEFILE_DELAY_UNTIL_REBOOT`.

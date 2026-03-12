# Service Management

After installation, the installer binary doubles as a Windows service manager. When launched with `--service`, it registers with the Windows Service Control Manager (SCM) and manages your application as a child process.

## How It Works

1. The installer copies itself to the install directory as `{slug}-setup.exe`.
2. A Windows service is registered that runs this binary with the `--service` flag.
3. On service start, the binary reads configuration, launches your app as a child process, and monitors it.

Your application does not need to implement any Windows service APIs. It is a normal process that reads environment variables and writes to stdout/stderr.

## Configuration Loading

At service start:

1. Reads `install-config.json` from the data directory (`%ProgramData%\{Name}\`)
2. Merges install-time variables with built-in variables
3. Expands `%var%` tokens in the `Command` field
4. Parses the expanded command into executable + arguments

## Child Process

The service launches your application with:

- **Working directory:** the data directory (`%ProgramData%\{Name}\`)
- **Environment variables:** all configuration variables are injected as uppercase env vars. For example, a variable `port` with value `8080` becomes `PORT=8080`. The full parent process environment is also inherited.
- **Process group:** the child runs in its own process group for clean shutdown signaling.

If the `Command` field uses a relative executable path, it is resolved relative to the install directory.

## Windows Event Log Integration

Stdout and stderr from your application are captured line by line. Each line is checked for a severity prefix:

| Prefix | Event Log Level |
|--------|----------------|
| `ERROR:` | Error |
| `WARNING:` | Warning |
| `INFO:` | Information |

Lines **with** a recognized prefix are written to both the log file and the Windows Event Log. Lines **without** a prefix are written to the log file only.

### Example

If your application prints:

```
ERROR: database connection failed
WARNING: cache miss for key "foo"
starting HTTP server on :8080
```

The first two lines appear in Event Viewer under your application's source name (at Error and Warning severity respectively). The third line is written to the log file only.

To produce Event Log entries from your application, simply print lines with the appropriate prefix:

```go
fmt.Println("ERROR: something went wrong")
fmt.Println("WARNING: disk space low")
fmt.Println("INFO: startup complete")
```

The prefix format is `SEVERITY: message` (severity in all caps, followed by a colon and space).

## Graceful Shutdown

When the service receives a Stop or Shutdown request:

1. Sends `CTRL_BREAK_EVENT` to the child's process group
2. Waits up to 10 seconds for the child to exit
3. If the child hasn't exited, kills it

Your application should handle `CTRL_BREAK` (or `SIGINT` on Go programs) to shut down cleanly within the 10-second window.

## Backoff Restart

If the child process exits unexpectedly, the service restarts it with exponential backoff:

| Attempt | Delay |
|---------|-------|
| 1 | 1 second |
| 2 | 2 seconds |
| 3 | 4 seconds |
| 4 | 8 seconds |
| 5 | 16 seconds |
| 6 | 32 seconds |
| 7+ | 5 minutes |

The backoff counter resets if the process runs for more than 1 minute, on the assumption that it reached a healthy state.

## Log Rotation

Service logs are written to `%ProgramData%\{Name}\logs\service.log`.

- Logs are rotated on service start (current log moved to `service.old.log`)
- Mid-session rotation occurs when the log file exceeds 10 MB
- One current log and one backup are kept

Log lines include timestamps in RFC 3339 format.

## What Your Application Does NOT Need

- No Windows service library imports -- ThePipes handles SCM integration
- No log rotation -- ThePipes rotates logs for you
- No Event Log registration -- ThePipes registers and writes to the Event Log
- No signal handling library -- just handle `CTRL_BREAK` for graceful shutdown
- No config file parsing -- read environment variables instead

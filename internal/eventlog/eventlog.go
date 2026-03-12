//go:build windows

package eventlog

import (
	"fmt"

	"golang.org/x/sys/windows/svc/eventlog"
)

// Register installs the application as a Windows Event Log source.
func Register(appName string) error {
	if err := eventlog.InstallAsEventCreate(appName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		// Already registered is not an error.
		return nil
	}
	return nil
}

// Unregister removes the Event Log source registration.
func Unregister(appName string) error {
	return eventlog.Remove(appName)
}

// Open opens the Event Log for writing under the given source name.
func Open(appName string) (*eventlog.Log, error) {
	return eventlog.Open(appName)
}

// Write writes a message to the Event Log with the given severity.
// Severity strings: "ERROR", "WARNING", "INFO".
func Write(log *eventlog.Log, severity, message string) error {
	if log == nil {
		return nil
	}
	switch severity {
	case "ERROR":
		return log.Error(1, message)
	case "WARNING":
		return log.Warning(2, message)
	default:
		return log.Info(3, message)
	}
}

// ParsePrefix checks if line begins with a recognized severity prefix.
// Returns the prefix (without the colon) and the remainder, or ("", line) if none.
func ParsePrefix(line string) (severity, rest string) {
	prefixes := []string{"ERROR", "WARNING", "INFO"}
	for _, p := range prefixes {
		if len(line) > len(p)+1 && line[:len(p)] == p && line[len(p)] == ':' {
			return p, fmt.Sprintf("[%s] %s", p, line[len(p)+1:])
		}
	}
	return "", line
}

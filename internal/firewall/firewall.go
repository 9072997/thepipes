//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
)

// Add adds an inbound allow firewall rule for the given executable.
func Add(appName, exePath string) error {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+appName,
		"dir=in",
		"action=allow",
		"program="+exePath,
		"enable=yes",
		"profile=any",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add firewall rule: %w\n%s", err, out)
	}
	return nil
}

// Remove deletes all firewall rules with the given name.
func Remove(appName string) error {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+appName,
	)
	// Ignore errors - the rule may not exist.
	_ = cmd.Run()
	return nil
}

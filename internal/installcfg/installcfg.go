package installcfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const filename = "install-config.json"

// Write serialises vars to %data%\install-config.json.
// Keys starting with "." are hidden and are not persisted.
func Write(dataDir string, vars map[string]string) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	filtered := make(map[string]string, len(vars))
	for k, v := range vars {
		if !strings.HasPrefix(k, ".") {
			filtered[k] = v
		}
	}
	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	path := filepath.Join(dataDir, filename)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// Read deserialises %data%\install-config.json.
func Read(dataDir string) (map[string]string, error) {
	path := filepath.Join(dataDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var vars map[string]string
	if err := json.Unmarshal(data, &vars); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return vars, nil
}

//go:build windows

package regutil

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const (
	uninstallBase = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`
	appBase       = `SOFTWARE`
	updateSubKey  = `Update`
)

// RegisterARP writes the Add/Remove Programs entry for the application.
func RegisterARP(appName, version, publisher, installDir, uninstallCmd string, estimatedSizeKB uint32) error {
	keyPath := uninstallBase + `\` + appName
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create ARP key: %w", err)
	}
	defer k.Close()

	set := func(name, value string) error {
		return k.SetStringValue(name, value)
	}
	if err := set("DisplayName", appName); err != nil {
		return err
	}
	if version != "" {
		if err := set("DisplayVersion", version); err != nil {
			return err
		}
	}
	if publisher != "" {
		if err := set("Publisher", publisher); err != nil {
			return err
		}
	}
	if err := set("InstallLocation", installDir); err != nil {
		return err
	}
	if err := set("UninstallString", uninstallCmd); err != nil {
		return err
	}
	if err := k.SetDWordValue("NoModify", 1); err != nil {
		return err
	}
	if err := k.SetDWordValue("NoRepair", 1); err != nil {
		return err
	}
	if estimatedSizeKB > 0 {
		if err := k.SetDWordValue("EstimatedSize", estimatedSizeKB); err != nil {
			return err
		}
	}
	return nil
}

// RemoveARP deletes the ARP registry entry for appName.
func RemoveARP(appName string) error {
	keyPath := uninstallBase + `\` + appName
	err := registry.DeleteKey(registry.LOCAL_MACHINE, keyPath)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete ARP key: %w", err)
	}
	return nil
}

// IsInstalled returns true if the ARP key for appName exists.
func IsInstalled(appName string) (bool, error) {
	keyPath := uninstallBase + `\` + appName
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	k.Close()
	return true, nil
}

// WriteUpdateState stores the update state under HKLM\SOFTWARE\{Name}\Update.
func WriteUpdateState(appName, hash, etag, lastModified string) error {
	keyPath := appBase + `\` + appName + `\` + updateSubKey
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("create Update key: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue("CurrentHash", hash); err != nil {
		return err
	}
	if err := k.SetStringValue("ETag", etag); err != nil {
		return err
	}
	return k.SetStringValue("LastModified", lastModified)
}

// ReadUpdateState reads the stored update state. Missing values are returned as empty strings.
func ReadUpdateState(appName string) (hash, etag, lastModified string, err error) {
	keyPath := appBase + `\` + appName + `\` + updateSubKey
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return "", "", "", nil
		}
		return "", "", "", fmt.Errorf("open Update key: %w", err)
	}
	defer k.Close()
	hash, _, _ = k.GetStringValue("CurrentHash")
	etag, _, _ = k.GetStringValue("ETag")
	lastModified, _, _ = k.GetStringValue("LastModified")
	return hash, etag, lastModified, nil
}

// RemoveUpdateState deletes HKLM\SOFTWARE\{Name}\Update.
func RemoveUpdateState(appName string) error {
	keyPath := appBase + `\` + appName + `\` + updateSubKey
	err := registry.DeleteKey(registry.LOCAL_MACHINE, keyPath)
	if err != nil && err != registry.ErrNotExist {
		return fmt.Errorf("delete Update key: %w", err)
	}
	// Also remove parent app key if empty.
	parentPath := appBase + `\` + appName
	_ = deleteKeyRecursive(registry.LOCAL_MACHINE, parentPath)
	return nil
}

// RemoveAppKey deletes HKLM\SOFTWARE\{Name} and all sub-keys.
func RemoveAppKey(appName string) error {
	keyPath := appBase + `\` + appName
	return deleteKeyRecursive(registry.LOCAL_MACHINE, keyPath)
}

func deleteKeyRecursive(k registry.Key, path string) error {
	key, err := registry.OpenKey(k, path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	subkeys, err := key.ReadSubKeyNames(-1)
	key.Close()
	if err != nil {
		return err
	}
	for _, sub := range subkeys {
		if err := deleteKeyRecursive(k, path+`\`+sub); err != nil {
			return err
		}
	}
	return registry.DeleteKey(k, path)
}

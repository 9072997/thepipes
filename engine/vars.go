package engine

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Expand replaces %key% tokens in s using vars.
func Expand(s string, vars map[string]string) string {
	for k, v := range vars {
		s = strings.ReplaceAll(s, "%"+k+"%", v)
	}
	return s
}

// BuiltinVars returns the default variables derived from the manifest.
func BuiltinVars(manifest AppManifest) map[string]string {
	vars := map[string]string{
		"install":   getInstallDir(manifest.Name),
		"data":      getDataDir(manifest.Name),
		"name":      manifest.Name,
		"publisher": manifest.Publisher,
		"version":   manifest.Version,
		"command":   manifest.Command,
	}
	if manifest.UpdateURL != "" {
		vars["update_url"] = manifest.UpdateURL
	}
	return vars
}

// MergeVars merges multiple maps left-to-right; later maps win.
func MergeVars(maps ...map[string]string) map[string]string {
	result := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

func getInstallDir(name string) string {
	pf := os.Getenv("ProgramFiles")
	if pf == "" {
		pf = `C:\Program Files`
	}
	return filepath.Join(pf, name)
}

func getDataDir(name string) string {
	pd := os.Getenv("ProgramData")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	return filepath.Join(pd, name)
}

// slug converts a display name to a service/binary identifier.
// "My Cool App" -> "my-cool-app"
func slug(name string) string {
	var sb strings.Builder
	prev := '-'
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
			prev = r
		} else if prev != '-' {
			sb.WriteRune('-')
			prev = '-'
		}
	}
	s := strings.TrimSuffix(sb.String(), "-")
	if s == "" {
		s = "app"
	}
	return s
}

// binaryName returns the installer binary filename for the app.
func binaryName(name string) string {
	return slug(name) + "-setup.exe"
}

// binaryPathInInstallDir returns the full path where this binary is copied during install.
func binaryPathInInstallDir(name string) string {
	return filepath.Join(getInstallDir(name), binaryName(name))
}

// firstToken returns the first whitespace-separated token from s,
// handling quoted paths.
func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}
	if s[0] == '"' {
		end := strings.Index(s[1:], `"`)
		if end >= 0 {
			return s[1 : end+1]
		}
	}
	sp := strings.IndexByte(s, ' ')
	if sp < 0 {
		return s
	}
	return s[:sp]
}

//go:build windows

package main

import (
	"fmt"
	"io"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
)

// newProcessFunc compiles tengo source into a function suitable for Process.Func.
// Scripts can signal failure by setting the error variable:
//
//	error = "something went wrong"
func newProcessFunc(src []byte) func(io.Writer, map[string]string) error {
	return func(log io.Writer, vars map[string]string) error {
		s := tengo.NewScript(src)
		s.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
		_ = s.Add("log", newTengoWriter(log))
		_ = s.Add("vars", toTengoMap(vars))
		_ = s.Add("error", "")

		compiled, err := s.Run()
		if err != nil {
			return fmt.Errorf("script error: %w", err)
		}
		syncTengoMap(compiled, "vars", vars)

		if msg := compiled.Get("error").String(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}

// newValidateFunc compiles tengo source into a function suitable for Simple.ValidateFunc.
func newValidateFunc(src []byte) func(map[string]string) error {
	return func(vars map[string]string) error {
		s := tengo.NewScript(src)
		s.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
		_ = s.Add("vars", toTengoMap(vars))
		_ = s.Add("error", "")

		compiled, err := s.Run()
		if err != nil {
			return fmt.Errorf("script error: %w", err)
		}
		syncTengoMap(compiled, "vars", vars)

		if msg := compiled.Get("error").String(); msg != "" {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}

// toTengoMap converts map[string]string to the map[string]interface{} that tengo requires.
func toTengoMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// syncTengoMap reads a named map variable from a compiled tengo script and
// syncs its string values back into dst. New keys are added; existing keys
// are updated; keys deleted by the script are removed from dst.
func syncTengoMap(c *tengo.Compiled, name string, dst map[string]string) {
	m := c.Get(name).Map()
	if m == nil {
		return
	}
	// Update and add keys.
	for k, v := range m {
		if s, ok := v.(string); ok {
			dst[k] = s
		}
	}
	// Remove keys that the script deleted.
	for k := range dst {
		if _, ok := m[k]; !ok {
			delete(dst, k)
		}
	}
}

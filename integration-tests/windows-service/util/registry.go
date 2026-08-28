//go:build windows

package util

import (
	"golang.org/x/sys/windows/registry"
)

// RegistryKeyExists returns true if the Alloy registry key exists under HKLM.
func RegistryKeyExists(registryPath string) bool {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.READ)
	if err != nil {
		return false
	}
	_ = k.Close()
	return true
}

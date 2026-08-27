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

// RegistryStringValue reads a REG_SZ value under HKLM at registryPath.
func RegistryStringValue(registryPath, name string) (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, registryPath, registry.READ)
	if err != nil {
		return "", err
	}
	defer k.Close()
	v, _, err := k.GetStringValue(name)
	return v, err
}

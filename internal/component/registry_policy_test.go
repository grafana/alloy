package component

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/stretchr/testify/require"
)

func makeTestRegistryMap() Registry {
	return NewRegistryMap(
		featuregate.StabilityGenerallyAvailable,
		false,
		map[string]Registration{
			"test.allowed": {
				Name:      "test.allowed",
				Stability: featuregate.StabilityGenerallyAvailable,
				Args:      nil,
				Build:     nil,
			},
			"test.blocked": {
				Name:      "test.blocked",
				Stability: featuregate.StabilityGenerallyAvailable,
				Args:      nil,
				Build:     nil,
			},
		},
	)
}

func TestPolicyRegistry_NilCheck(t *testing.T) {
	inner := makeTestRegistryMap()
	reg := NewPolicyFilteredRegistry(inner, nil)

	_, err := reg.Get("test.allowed")
	require.NoError(t, err)
}

func TestPolicyRegistry_CheckAllows(t *testing.T) {
	inner := makeTestRegistryMap()
	check := func(name string) error {
		if name == "test.blocked" {
			return errors.New("blocked by policy")
		}
		return nil
	}
	reg := NewPolicyFilteredRegistry(inner, check)

	_, err := reg.Get("test.allowed")
	require.NoError(t, err)

	_, err = reg.Get("test.blocked")
	require.Error(t, err)
	require.Contains(t, err.Error(), "blocked by policy")
}

func TestPolicyRegistry_CheckRunsBeforeInner(t *testing.T) {
	// Even if the component doesn't exist in inner, the check error should
	// be returned (check is the first gate).
	inner := makeTestRegistryMap()
	check := func(name string) error {
		return errors.New("policy says no")
	}
	reg := NewPolicyFilteredRegistry(inner, check)

	_, err := reg.Get("test.allowed")
	require.Error(t, err)
	require.Contains(t, err.Error(), "policy says no")
}

func TestPolicyRegistry_InnerErrorPassedThrough(t *testing.T) {
	inner := makeTestRegistryMap()
	check := func(name string) error { return nil }
	reg := NewPolicyFilteredRegistry(inner, check)

	_, err := reg.Get("nonexistent.component")
	require.Error(t, err)
}

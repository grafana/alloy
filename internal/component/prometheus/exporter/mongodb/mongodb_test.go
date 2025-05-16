package mongodb

import (
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = false // Override default
	enable_coll_stats = false // Override default
	`

	var args Arguments
	args.SetToDefault() // Apply new defaults (DirectConnect=true, others=false)
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		URI:             "mongodb://127.0.0.1:27017",
		DirectConnect:   true,  // Explicitly set, matches default
		DiscoveringMode: false, // Explicitly set, matches default
		EnableCollStats: false, // Explicitly set, matches default

		// Fields that should retain their default 'false' value from SetToDefault
		CompatibleMode:           false,
		CollectAll:               false,
		EnableDBStats:            false,
		EnableDBStatsFreeStorage: false,
		EnableDiagnosticData:     false,
		EnableReplicasetStatus:   false,
		EnableReplicasetConfig:   false,
		EnableCurrentopMetrics:   false,
		EnableTopMetrics:         false,
		EnableIndexStats:         false,
		EnableProfile:            false,
		EnableShards:             false,
		EnableFCV:                false,
		EnablePBMMetrics:         false,
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	alloyConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = false // Override default
	enable_db_stats = false // Override default
	`
	var args Arguments
	args.SetToDefault() // Apply new defaults
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := mongodb_exporter.Config{
		URI: "mongodb://127.0.0.1:27017",
		// Explicitly set values
		DirectConnect:   true,  // Explicitly set, matches default
		DiscoveringMode: false, // Explicitly set, matches default
		EnableDBStats:   false, // Explicitly set, overrides default false (no change from default)

		// Default values (false, unless DirectConnect)
		CompatibleMode:           false,
		CollectAll:               false,
		EnableDBStatsFreeStorage: false,
		EnableDiagnosticData:     false,
		EnableReplicasetStatus:   false,
		EnableReplicasetConfig:   false,
		EnableCurrentopMetrics:   false,
		EnableTopMetrics:         false,
		EnableIndexStats:         false,
		EnableCollStats:          false, // Default is false
		EnableProfile:            false,
		EnableShards:             false,
		EnableFCV:                false,
		EnablePBMMetrics:         false,
	}
	require.Equal(t, expected, *res)
}

func TestAlloyUnmarshal_Defaults(t *testing.T) {
	alloyConfig := `mongodb_uri = "mongodb://127.0.0.1:27017"`

	var args Arguments
	args.SetToDefault() // Apply new defaults
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	// Check defaults: DirectConnect true, others false
	expectedArgs := Arguments{
		URI:                      "mongodb://127.0.0.1:27017",
		DirectConnect:            true,
		CompatibleMode:           false,
		CollectAll:               false,
		DiscoveringMode:          false,
		EnableDBStats:            false,
		EnableDBStatsFreeStorage: false,
		EnableDiagnosticData:     false,
		EnableReplicasetStatus:   false,
		EnableReplicasetConfig:   false,
		EnableCurrentopMetrics:   false,
		EnableTopMetrics:         false,
		EnableIndexStats:         false,
		EnableCollStats:          false,
		EnableProfile:            false,
		EnableShards:             false,
		EnableFCV:                false,
		EnablePBMMetrics:         false,
	}
	require.Equal(t, expectedArgs, args)
}

func TestConvert_CollectAllTrue_EnableCollStatsFalse(t *testing.T) {
	args := Arguments{
		CollectAll:      true,  // User scenario
		EnableCollStats: false, // User scenario
		// Other fields will be their Go zero value (false for bools) as SetToDefault is not called here.
	}

	cfg := args.Convert()
	require.True(t, cfg.CollectAll, "CollectAll should be true")
	require.False(t, cfg.EnableCollStats, "EnableCollStats should be false")
}

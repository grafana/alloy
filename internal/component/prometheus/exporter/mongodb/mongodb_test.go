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
	discovering_mode = true
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		URI:             "mongodb://127.0.0.1:27017",
		LogLevel:        "info",
		DirectConnect:   true,
		DiscoveringMode: true,
		CompatibleMode:  true,
		CollectAll:      true,
	}

	require.Equal(t, expected, args)
}

func TestConvert(t *testing.T) {
	alloyConfig := `
	mongodb_uri = "mongodb://127.0.0.1:27017"
	direct_connect = true
	discovering_mode = true
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := mongodb_exporter.Config{
		URI:             "mongodb://127.0.0.1:27017",
		DirectConnect:   true,
		DiscoveringMode: true,
		CompatibleMode:  true,
		CollectAll:      true,
	}
	_ = expected.LogLevel.Set("info")
	require.Equal(t, expected, *res)
}

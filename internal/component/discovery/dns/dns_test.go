package dns

import (
	"strings"
	"testing"
	"time"

	"github.com/grafana/river"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestRiverUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
	refresh_interval = "5m"
	port = 54
	names = ["foo.com"]
	type = "A"
`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	assert.Equal(t, 5*time.Minute, args.RefreshInterval)
	assert.Equal(t, 54, args.Port)
	assert.Equal(t, "foo.com", strings.Join(args.Names, ","))
}

func TestBadRiverConfig(t *testing.T) {
	var tests = []struct {
		Desc   string
		Config string
	}{
		{
			Desc:   "No Name",
			Config: "",
		},
		{
			Desc: "Bad Type",
			Config: `names = ["example"]
			type = "CNAME"`,
		},
		{
			Desc: "A without port",
			Config: `names = ["example"]
			type = "A"`,
		},
		{
			Desc: "AAAA without port",
			Config: `names = ["example"]
			type = "AAAA"`,
		},
	}
	for _, tst := range tests {
		cfg := tst.Config
		t.Run(tst.Desc, func(t *testing.T) {
			var args Arguments
			err := river.Unmarshal([]byte(cfg), &args)
			require.Error(t, err)
		})
	}
}

func TestConvert(t *testing.T) {
	args := Arguments{
		RefreshInterval: 5 * time.Minute,
		Port:            8181,
		Type:            "A",
		Names:           []string{"example.com", "example2.com"},
	}

	converted := args.Convert()
	assert.Equal(t, model.Duration(5*time.Minute), converted.RefreshInterval)
	assert.Equal(t, 8181, converted.Port)
	assert.Equal(t, "A", converted.Type)
	assert.Equal(t, "example.com,example2.com", strings.Join(converted.Names, ","))
}

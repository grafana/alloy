package syntax_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
)

type upstreamTLS struct {
	Insecure bool   `mapstructure:"insecure"`
	CAFile   string `mapstructure:"ca_file"`
}

type upstreamClient struct {
	Endpoint string        `mapstructure:"endpoint"`
	Timeout  time.Duration `mapstructure:"timeout"`
	TLS      upstreamTLS   `mapstructure:"tls"`
}

type upstreamConfig struct {
	upstreamClient `mapstructure:",squash"`

	Headers map[string]string `mapstructure:"headers"`
	Rules   []upstreamRule    `mapstructure:"rules"`
}

type upstreamRule struct {
	Match string `mapstructure:"match"`
}

// wrapper mimics how an Alloy component wraps an upstream config: the upstream
// struct is squashed in using mapstructure tags, next to Alloy-only fields.
type wrapper struct {
	Config upstreamConfig `alloy:",squash,mapstructure"`
	Label  string         `alloy:"label,attr,optional"`
}

func (w *wrapper) SetToDefault() {
	w.Config = upstreamConfig{upstreamClient: upstreamClient{
		Timeout: 5 * time.Second,
		TLS:     upstreamTLS{CAFile: "/default/ca.pem"},
	}}
}

func TestMapstructure_Decode(t *testing.T) {
	in := `
		endpoint = "localhost:4317"
		headers  = { "X-Scope-OrgID" = "tenant" }
		label    = "alloy-only"

		tls {
			insecure = true
		}

		rules { match = "a" }
		rules { match = "b" }
	`

	var actual wrapper
	require.NoError(t, syntax.Unmarshal([]byte(in), &actual))

	require.Equal(t, wrapper{
		Config: upstreamConfig{
			upstreamClient: upstreamClient{
				Endpoint: "localhost:4317",
				Timeout:  5 * time.Second, // Default kept.
				// The nested block keeps defaults for fields it doesn't set.
				TLS: upstreamTLS{Insecure: true, CAFile: "/default/ca.pem"},
			},
			Headers: map[string]string{"X-Scope-OrgID": "tenant"},
			Rules:   []upstreamRule{{Match: "a"}, {Match: "b"}},
		},
		Label: "alloy-only",
	}, actual)
}

func TestMapstructure_DecodeErrors(t *testing.T) {
	var actual wrapper
	err := syntax.Unmarshal([]byte(`not_a_field = 1`), &actual)
	require.ErrorContains(t, err, `unrecognized attribute name "not_a_field"`)

	err = syntax.Unmarshal([]byte(`tls = {}`), &actual)
	require.ErrorContains(t, err, `"tls" must be a block, but is used as an attribute`)
}

func TestMapstructure_Encode(t *testing.T) {
	var w wrapper
	w.SetToDefault()
	w.Config.Endpoint = "localhost:4317"
	w.Config.TLS.Insecure = true
	w.Config.Rules = []upstreamRule{{Match: "a"}}

	out, err := syntax.Marshal(w)
	require.NoError(t, err)
	// Top-level values equal to the defaults are omitted, as for alloy tags.
	require.Equal(t, `endpoint = "localhost:4317"

tls {
	insecure = true
	ca_file  = "/default/ca.pem"
}

rules {
	match = "a"
}`, string(out))
}

func TestBlockMap(t *testing.T) {
	in := `
		endpoint = "localhost:4317"
		tls {
			insecure = true
		}
		rules { match = "a" }
		rules { match = "b" }
	`

	var actual syntax.BlockMap
	require.NoError(t, syntax.Unmarshal([]byte(in), &actual))
	require.Equal(t, syntax.BlockMap{
		"endpoint": "localhost:4317",
		"tls":      map[string]any{"insecure": true},
		"rules": []any{
			map[string]any{"match": "a"},
			map[string]any{"match": "b"},
		},
	}, actual)

	err := syntax.Unmarshal([]byte("tls = 1\ntls {}"), &actual)
	require.ErrorContains(t, err, `"tls" is used as both an attribute and a block`)
}

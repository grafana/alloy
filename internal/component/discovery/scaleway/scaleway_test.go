package scaleway

import (
	"testing"

	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	tt := []struct {
		name   string
		config string
	}{
		{
			name: "required attributes only",
			config: `
				project_id = "00000000-0000-0000-0000-000000000000"
				role       = "baremetal"
				access_key = "SCWXXXXXXXXXXXXXXXXX"
				secret_key = "00000000-0000-0000-0000-000000000000"
			`,
		},
		{
			name: "multiple optional attributes",
			config: `
				project_id       = "00000000-0000-0000-0000-000000000000"
				role             = "instance"
				api_url          = "http://custom.api.local"
				access_key       = "SCWXXXXXXXXXXXXXXXXX"
				secret_key       = "00000000-0000-0000-0000-000000000000"
				name_filter      = "foo"
				tags_filter      = ["foo", "bar"]
				refresh_interval = "5m"
				port             = 1234
			`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var args Arguments
			err := river.Unmarshal([]byte(tc.config), &args)
			require.NoError(t, err)

			// Assert that args.Convert() doesn't panic.
			require.NotPanics(t, func() {
				_ = args.Convert()
			})
		})
	}
}

func TestUnsafeCast(t *testing.T) {
	// Scaleway has some hidden validations:
	//
	// * project_id and secret_key are always UUIDs
	// * access_key is always formatted as SCWXXXXXXXXXXXXXXXXX
	//
	// If we break one of those assumptions above, then the upstream code which
	// validates our inputs based on the scaleway SDK should return an error,
	// provided our unsafe cast works properly.
	input := `
		project_id = "invalid_id"
		role       = "baremetal"
		access_key = "SCWXXXXXXXXXXXXXXXXX"
		secret_key = "00000000-0000-0000-0000-000000000000"
	`
	var args Arguments
	err := river.Unmarshal([]byte(input), &args)
	require.ErrorContains(t, err, "invalid project ID format")
}

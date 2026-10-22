package prometheus

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/prometheus/internal/convert"
	"github.com/stretchr/testify/require"
)

// TestConvertArgumentsToConvertOptions covers the translation from the
// component's arguments to the converter's options. Every attribute of
// Arguments that the converter understands has to be copied here: a field that
// is missing is accepted by the config decoder, documented, and then silently
// ignored (#7459).
func TestConvertArgumentsToConvertOptions(t *testing.T) {
	tests := []struct {
		name string
		args Arguments
		want convert.Options
	}{
		{
			name: "defaults",
			args: DefaultArguments,
			want: convert.Options{
				IncludeTargetInfo:  true,
				IncludeScopeLabels: true,
				AddMetricSuffixes:  true,
			},
		},
		{
			name: "explicit values",
			args: Arguments{
				IncludeTargetInfo:                 false,
				IncludeScopeInfo:                  true,
				IncludeScopeLabels:                false,
				AddMetricSuffixes:                 false,
				ResourceToTelemetryConversion:     true,
				HonorMetadata:                     true,
				ConvertClassicHistogramsToNHCB:    true,
				KeepIdentifyingResourceAttributes: true,
				StartTimestampZeroIngestion:       true,
			},
			want: convert.Options{
				IncludeTargetInfo:                 false,
				IncludeScopeInfo:                  true,
				IncludeScopeLabels:                false,
				AddMetricSuffixes:                 false,
				ResourceToTelemetryConversion:     true,
				HonorMetadata:                     true,
				ConvertClassicHistogramsToNHCB:    true,
				KeepIdentifyingResourceAttributes: true,
				EnableStartTimestampZeroIngestion: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, convertArgumentsToConvertOptions(tc.args))
		})
	}
}

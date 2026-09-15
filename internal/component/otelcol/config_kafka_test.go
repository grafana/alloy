package otelcol

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestKafkaAuthenticationArguments_LogDeprecations(t *testing.T) {
	tt := []struct {
		name string
		args KafkaAuthenticationArguments
		want string
	}{
		{
			name: "nothing set",
			args: KafkaAuthenticationArguments{},
			want: "",
		},
		{
			name: "plaintext set",
			args: KafkaAuthenticationArguments{Plaintext: &KafkaPlaintextArguments{Username: "u", Password: alloytypes.Secret("p")}},
			want: "authentication.plaintext",
		},
		{
			name: "sasl version set",
			args: KafkaAuthenticationArguments{SASL: &KafkaSASLArguments{Version: 1}},
			want: "authentication.sasl.version",
		},
		{
			name: "sasl without version",
			args: KafkaAuthenticationArguments{SASL: &KafkaSASLArguments{Mechanism: "PLAIN"}},
			want: "",
		},
		{
			name: "tls set",
			args: KafkaAuthenticationArguments{TLS: &TLSClientArguments{}},
			want: "authentication.tls",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			tc.args.LogDeprecations(logger)

			if tc.want == "" {
				require.Empty(t, buf.String())
			} else {
				require.Contains(t, buf.String(), tc.want)
			}
		})
	}
}

func TestKafkaAuthenticationArguments_LogDeprecations_nilLogger(t *testing.T) {
	require.NotPanics(t, func() {
		KafkaAuthenticationArguments{Plaintext: &KafkaPlaintextArguments{}}.LogDeprecations(nil)
	})
}

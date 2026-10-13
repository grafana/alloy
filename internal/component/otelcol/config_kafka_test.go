package otelcol

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKafkaAuthenticationArguments_LogDeprecations(t *testing.T) {
	args := KafkaAuthenticationArguments{TLS: &TLSClientArguments{}}

	var buf bytes.Buffer
	args.LogDeprecations(slog.New(slog.NewTextHandler(&buf, nil)))
	require.NotEmpty(t, buf.String())

	require.NotPanics(t, func() {
		args.LogDeprecations(nil)
	})
}

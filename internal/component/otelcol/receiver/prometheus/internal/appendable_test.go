package internal

import (
	"context"
	"testing"

	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

// prometheus.receive_http writes through AppenderV2, so it must not panic and
// must support exemplars.
func TestAppendable_AppenderV2(t *testing.T) {
	var o appendable

	var app storage.AppenderV2
	require.NotPanics(t, func() { app = o.AppenderV2(context.Background()) })
	require.NotNil(t, app)
	require.Implements(t, (*storage.ExemplarAppenderV2)(nil), app)
}

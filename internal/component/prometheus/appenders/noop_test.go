package appenders

import (
	"testing"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestNoopV2_AppendExemplars(t *testing.T) {
	var app storage.AppenderV2 = NoopV2{}
	ea, ok := app.(storage.ExemplarAppenderV2)
	require.True(t, ok)

	ref, err := ea.AppendExemplars(7, labels.FromStrings("job", "test"), []exemplar.Exemplar{{Value: 1}})
	require.NoError(t, err)
	require.Equal(t, storage.SeriesRef(7), ref)
}

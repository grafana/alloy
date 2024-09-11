package serialization

import (
	"context"
	log2 "github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAppenderTTL(t *testing.T) {
	fake := &counterSerializer{}
	l := log2.NewNopLogger()

	app := NewAppender(context.Background(), 1*time.Minute, fake, l)
	_, err := app.Append(0, labels.FromStrings("one", "two"), time.Now().Unix(), 0)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err = app.Append(0, labels.FromStrings("one", "two"), time.Now().Add(-5*time.Minute).Unix(), 0)
		require.NoError(t, err)
	}
	// Only one record should make it through.
	require.True(t, fake.received == 1)
}

var _ types.Serializer = (*fakeSerializer)(nil)

type counterSerializer struct {
	received int
}

func (f *counterSerializer) Start() {

}

func (f *counterSerializer) Stop() {

}

func (f *counterSerializer) SendSeries(ctx context.Context, data *types.TimeSeriesBinary) error {
	f.received++
	return nil

}

func (f *counterSerializer) SendMetadata(ctx context.Context, data *types.TimeSeriesBinary) error {
	return nil
}

func (f *counterSerializer) UpdateConfig(ctx context.Context, data types.SerializerConfig) error {
	return nil
}

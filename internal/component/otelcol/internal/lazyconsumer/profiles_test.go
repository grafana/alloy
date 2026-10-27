package lazyconsumer

import (
	"context"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/pipeline"
)

func TestProfilesClonesForMutatingConsumer(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	c := New(ctx, "test")
	profiles := testdata.GenerateProfiles(1)
	profiles.MarkReadOnly()
	require.ErrorIs(t, c.ConsumeProfiles(ctx, profiles), pipeline.ErrSignalNotSupported)
	called := false
	c.SetConsumers(nil, nil, nil, &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities { return consumer.Capabilities{MutatesData: true} },
		ConsumeProfilesFunc: func(_ context.Context, pd pprofile.Profiles) error {
			called = true
			pd.ResourceProfiles().At(0).Resource().Attributes().PutStr("mutated", "true")
			return nil
		},
	})
	require.NoError(t, c.ConsumeProfiles(ctx, profiles))
	require.True(t, called)
	_, mutated := profiles.ResourceProfiles().At(0).Resource().Attributes().Get("mutated")
	require.False(t, mutated)
	c.SetConsumers(nil, nil, nil, nil)
	require.ErrorIs(t, c.ConsumeProfiles(ctx, profiles), pipeline.ErrSignalNotSupported)
	cancel()
	require.ErrorIs(t, c.ConsumeProfiles(ctx, profiles), context.Canceled)
}

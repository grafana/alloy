package fanoutconsumer

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
)

func TestProfilesClonesForMutatingConsumers(t *testing.T) {
	profiles := testdata.GenerateProfiles(1)
	mutating := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: true}
		},
		ConsumeProfilesFunc: func(_ context.Context, profiles pprofile.Profiles) error {
			profiles.ResourceProfiles().At(0).Resource().Attributes().PutStr("mutated", "true")
			return nil
		},
	}
	passthrough := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: false}
		},
		ConsumeProfilesFunc: func(_ context.Context, profiles pprofile.Profiles) error {
			_, ok := profiles.ResourceProfiles().At(0).Resource().Attributes().Get("mutated")
			require.False(t, ok)
			return nil
		},
	}

	fanout := Profiles([]otelcol.Consumer{mutating, passthrough})
	require.NoError(t, fanout.ConsumeProfiles(t.Context(), profiles))

	_, ok := profiles.ResourceProfiles().At(0).Resource().Attributes().Get("mutated")
	require.False(t, ok)
}

func TestProfilesReportsMutatingWhenAllConsumersMutate(t *testing.T) {
	first := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: true}
		},
	}
	second := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: true}
		},
	}

	fanout := Profiles([]otelcol.Consumer{first, second})
	require.True(t, fanout.Capabilities().MutatesData)
}

func TestProfilesClonesReadOnlyDataForMutatingConsumer(t *testing.T) {
	profiles := testdata.GenerateProfiles(1)
	profiles.MarkReadOnly()
	mutating := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: true}
		},
		ConsumeProfilesFunc: func(_ context.Context, profiles pprofile.Profiles) error {
			require.False(t, profiles.IsReadOnly())
			profiles.ResourceProfiles().At(0).Resource().Attributes().PutStr("mutated", "true")
			return nil
		},
	}
	anotherMutating := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: true}
		},
	}

	require.NoError(t, Profiles([]otelcol.Consumer{mutating, anotherMutating}).ConsumeProfiles(t.Context(), profiles))
	_, ok := profiles.ResourceProfiles().At(0).Resource().Attributes().Get("mutated")
	require.False(t, ok)
}

func TestProfilesMarksSharedDataReadOnly(t *testing.T) {
	profiles := testdata.GenerateProfiles(1)
	first := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: false}
		},
		ConsumeProfilesFunc: func(_ context.Context, profiles pprofile.Profiles) error {
			require.True(t, profiles.IsReadOnly())
			return nil
		},
	}
	second := &fakeconsumer.Consumer{
		CapabilitiesFunc: func() consumer.Capabilities {
			return consumer.Capabilities{MutatesData: false}
		},
		ConsumeProfilesFunc: func(_ context.Context, profiles pprofile.Profiles) error {
			require.True(t, profiles.IsReadOnly())
			return nil
		},
	}

	require.NoError(t, Profiles([]otelcol.Consumer{first, second}).ConsumeProfiles(t.Context(), profiles))
	require.True(t, profiles.IsReadOnly())
}

func TestProfilesAggregatesErrors(t *testing.T) {
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	first := &fakeconsumer.Consumer{
		ConsumeProfilesFunc: func(context.Context, pprofile.Profiles) error {
			return firstErr
		},
	}
	second := &fakeconsumer.Consumer{
		ConsumeProfilesFunc: func(context.Context, pprofile.Profiles) error {
			return secondErr
		},
	}

	err := Profiles([]otelcol.Consumer{first, second}).ConsumeProfiles(t.Context(), testdata.GenerateProfiles(1))
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, secondErr)
}

func TestProfilesIgnoresNilConsumers(t *testing.T) {
	called := false
	valid := &fakeconsumer.Consumer{
		ConsumeProfilesFunc: func(context.Context, pprofile.Profiles) error {
			called = true
			return nil
		},
	}

	require.NoError(t, Profiles([]otelcol.Consumer{nil}).ConsumeProfiles(t.Context(), pprofile.NewProfiles()))
	require.NoError(t, Profiles([]otelcol.Consumer{nil, valid, nil}).ConsumeProfiles(t.Context(), pprofile.NewProfiles()))
	require.True(t, called)
}

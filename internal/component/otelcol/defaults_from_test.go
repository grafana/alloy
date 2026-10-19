package otelcol_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/processor/tail_sampling"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
)

type upInner struct {
	Level int `mapstructure:"level"`
}

type UpSquashed struct {
	Shared string `mapstructure:"shared"`
}

type upCfg struct {
	UpSquashed `mapstructure:",squash"`

	Name     string                           `mapstructure:"name"`
	Timeout  time.Duration                    `mapstructure:"timeout"`
	Token    configopaque.String              `mapstructure:"token"`
	Tags     []string                         `mapstructure:"tags"`
	LimitMiB uint32                           `mapstructure:"limit_mib"`
	Skipped  string                           `mapstructure:"-"`
	Inner    upInner                          `mapstructure:"inner"`
	Some     configoptional.Optional[upInner] `mapstructure:"some"`
	None     configoptional.Optional[upInner] `mapstructure:"none"`
	Ptr      *upInner                         `mapstructure:"ptr"`
	Unmapped string                           `mapstructure:"unmapped"`
}

type alInner struct {
	Level int `alloy:"level,attr,optional"`
}

type AlSquashed struct {
	Shared string `alloy:"shared,attr,optional"`
}

type alArgs struct {
	AlSquashed `alloy:",squash"`

	Name      string            `alloy:"name,attr,optional"`
	Timeout   time.Duration     `alloy:"timeout,attr,optional"`
	Token     alloytypes.Secret `alloy:"token,attr,optional"`
	Tags      []string          `alloy:"tags,attr,optional"`
	Limit     uint64            `alloy:"limit,attr,optional"`
	Skipped   string            `alloy:"skipped,attr,optional"`
	Inner     alInner           `alloy:"inner,block,optional"`
	Some      *alInner          `alloy:"some,block,optional"`
	None      *alInner          `alloy:"none,block,optional"`
	Ptr       *alInner          `alloy:"ptr,block,optional"`
	AlloyOnly string            `alloy:"alloy_only,attr,optional"`
}

func newUp() *upCfg {
	return &upCfg{
		UpSquashed: UpSquashed{Shared: "s"},
		Name:       "n",
		Timeout:    time.Second,
		Token:      "secret",
		Tags:       []string{"a", "b"},
		LimitMiB:   4,
		Skipped:    "skip",
		Inner:      upInner{Level: 1},
		Some:       configoptional.Some(upInner{Level: 2}),
		None:       configoptional.None[upInner](),
		Ptr:        &upInner{Level: 3},
		Unmapped:   "x",
	}
}

func TestDefaultsFrom(t *testing.T) {
	up := newUp()
	args := alArgs{AlloyOnly: "keep"}
	err := otelcol.DefaultsFrom(up, &args, otelcol.WithRename("limit_mib", "limit", func(v reflect.Value) (any, error) {
		return v.Uint() * 1024 * 1024, nil
	}))
	require.NoError(t, err)

	require.Equal(t, "s", args.Shared)
	require.Equal(t, "n", args.Name)
	require.Equal(t, time.Second, args.Timeout)
	require.Equal(t, alloytypes.Secret("secret"), args.Token)
	require.Equal(t, []string{"a", "b"}, args.Tags)
	require.Equal(t, uint64(4*1024*1024), args.Limit)
	require.Empty(t, args.Skipped)
	require.Equal(t, 1, args.Inner.Level)
	// Pointer (optional block) fields are never allocated.
	require.Nil(t, args.Some)
	require.Nil(t, args.None)
	require.Nil(t, args.Ptr)
	require.Equal(t, "keep", args.AlloyOnly)

	// No aliasing of upstream slices.
	args.Tags[0] = "changed"
	require.Equal(t, "a", up.Tags[0])
}

func TestDefaultsFrom_Errors(t *testing.T) {
	type badArgs struct {
		Inner string `alloy:"inner,attr,optional"`
	}
	err := otelcol.DefaultsFrom(newUp(), &badArgs{})
	require.ErrorContains(t, err, "inner: cannot map upstream")

	type nested struct {
		Inner struct {
			Level string `alloy:"level,attr,optional"`
		} `alloy:"inner,block,optional"`
	}
	err = otelcol.DefaultsFrom(newUp(), &nested{})
	require.ErrorContains(t, err, "inner.level: cannot map upstream int to Alloy string")

	require.ErrorContains(t, otelcol.DefaultsFrom(newUp(), &alArgs{}, otelcol.WithRename("nope", "limit", nil)), "limit: renamed upstream field")
	require.Error(t, otelcol.DefaultsFrom(newUp(), alArgs{}))
}

func TestDefaultsFrom_RealUpstream(t *testing.T) {
	cfg := tailsamplingprocessor.NewFactory().CreateDefaultConfig().(*tailsamplingprocessor.Config)

	var args tail_sampling.Arguments
	require.NoError(t, otelcol.DefaultsFrom(cfg, &args))
	require.Equal(t, cfg.DecisionWait, args.DecisionWait)
	require.Equal(t, cfg.NumTraces, args.NumTraces)
	require.Equal(t, cfg.NumShards, args.NumShards)
	// Upstream has no policies by default, so the repeated block stays empty.
	require.Nil(t, args.PolicyCfgs)
}

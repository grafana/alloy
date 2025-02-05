package relabel

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/grafana/regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"
)

func TestRelabeling(t *testing.T) {
	tests := []struct {
		name        string
		rules       []*alloy_relabel.Config
		inputLabels labels.Labels
		wantLabels  labels.Labels
		wantDropped bool
	}{
		{
			name: "basic profile without labels",
			rules: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"foo"},
					TargetLabel:  "bar",
					Action:       "replace",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
					Replacement:  "$1",
				},
			},
			inputLabels: labels.EmptyLabels(),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: false,
		},
		{
			name: "rename label",
			rules: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"foo"},
					TargetLabel:  "bar",
					Action:       "replace",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
					Replacement:  "$1",
				},
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("foo")},
				},
			},
			inputLabels: labels.FromStrings("foo", "hello"),
			wantLabels:  labels.FromStrings("bar", "hello"),
			wantDropped: false,
		},
		{
			name: "drop profile with matching drop label",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "drop",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("dev")},
			}},
			inputLabels: labels.FromStrings("env", "dev", "region", "us-1"),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: true,
		},
		{
			name: "keep profile with matching label",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "keep",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("prod")},
			}},
			inputLabels: labels.FromStrings("env", "prod", "region", "us-1"),
			wantLabels:  labels.FromStrings("env", "prod", "region", "us-1"),
			wantDropped: false,
		},
		{
			name: "drop profile not matching keep",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "keep",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("prod")},
			}},
			inputLabels: labels.FromStrings("env", "dev", "region", "us-1"),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: true,
		},
		{
			name: "drop all labels not dropping profile",
			rules: []*alloy_relabel.Config{
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("env|region")},
				},
			},
			inputLabels: labels.FromStrings("env", "prod", "region", "us-1"),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: false,
		},
		{
			name: "keep profile with no labels when using drop action",
			rules: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"env"},
					Action:       "drop",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("dev")},
				},
			},
			inputLabels: labels.EmptyLabels(),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: false,
		},
		{
			name: "hashmod sampling with profile that should pass",
			rules: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"env"},
					Action:       "hashmod",
					Modulus:      2,
					TargetLabel:  "__tmp_hash",
				},
				{
					SourceLabels: []string{"__tmp_hash"},
					Action:       "drop",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("^1$")},
				},
			},
			// Use a value we know will hash to 1
			inputLabels: labels.FromStrings("env", "prod", "region", "us-1"),
			wantLabels:  labels.EmptyLabels(),
			wantDropped: true,
		},
		{
			name: "multiple rules",
			rules: []*alloy_relabel.Config{
				{
					SourceLabels: []string{"env"},
					TargetLabel:  "environment",
					Action:       "replace",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
					Replacement:  "$1",
				},
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("^env$")},
				},
				{
					SourceLabels: []string{"region"},
					TargetLabel:  "zone",
					Action:       "replace",
					Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("us-(.+)")},
					Replacement:  "zone-$1",
				},
			},
			inputLabels: labels.FromStrings("env", "prod", "region", "us-1"),
			wantLabels:  labels.FromStrings("environment", "prod", "region", "us-1", "zone", "zone-1"),
			wantDropped: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewTestAppender()

			c, err := New(component.Options{
				Logger:        util.TestLogger(t),
				Registerer:    prometheus.NewRegistry(),
				OnStateChange: func(e component.Exports) {},
			}, Arguments{
				ForwardTo:      []pyroscope.Appendable{app},
				RelabelConfigs: tt.rules,
				MaxCacheSize:   10,
			})
			require.NoError(t, err)

			profile := &pyroscope.IncomingProfile{
				Labels: tt.inputLabels,
			}

			err = c.AppendIngest(context.Background(), profile)

			profiles := app.Profiles()

			if tt.wantDropped {
				if errors.Is(err, labelset.ErrServiceNameIsRequired) {
					require.Empty(t, profiles, "profile should have been dropped")
					return
				}
				require.NoError(t, err)
				require.Empty(t, profiles, "profile should have been dropped")
				return
			}

			gotProfile := app.Profiles()[0]
			require.Equal(t, tt.wantLabels, gotProfile.Labels)
		})
	}
}

func TestCache(t *testing.T) {
	app := NewTestAppender()
	c, err := New(component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}, Arguments{
		ForwardTo: []pyroscope.Appendable{app},
		RelabelConfigs: []*alloy_relabel.Config{{
			SourceLabels: []string{"env"},
			Action:       "replace",
			TargetLabel:  "environment",
			Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("(.+)")},
			Replacement:  "staging",
		}},
		MaxCacheSize: 4,
	})
	require.NoError(t, err)

	// Test basic cache functionality
	labels := labels.FromStrings("env", "prod")
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: labels})
	require.NoError(t, err)
	require.Equal(t, 1, c.cache.Len(), "cache should have 1 entry")

	// Test cache hit
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: labels})
	require.NoError(t, err)
	require.Equal(t, 1, c.cache.Len(), "cache length should not change after hit")
}

func TestCacheCollisions(t *testing.T) {
	app := NewTestAppender()
	c, err := New(component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}, Arguments{
		ForwardTo:      []pyroscope.Appendable{app},
		RelabelConfigs: []*alloy_relabel.Config{},
		MaxCacheSize:   4,
	})
	require.NoError(t, err)

	// These LabelSets are known to collide
	ls1 := labels.FromStrings("A", "K6sjsNNczPl", "__name__", "app.cpu")
	ls2 := labels.FromStrings("A", "cswpLMIZpwt", "__name__", "app.cpu")

	// Verify collision
	require.Equal(t, toModelLabelSet(ls1).Fingerprint(), toModelLabelSet(ls2).Fingerprint(),
		"expected labelset fingerprints to collide")

	// Add both colliding profiles
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: ls1})
	require.NoError(t, err)
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: ls2})
	require.NoError(t, err)

	// Verify both are stored under same hash
	hash := toModelLabelSet(ls1).Fingerprint()
	val, ok := c.cache.Get(hash)
	require.True(t, ok, "colliding entry should be in cache")
	require.Len(t, val, 2, "should have both colliding items under same key")

	// Verify items are stored correctly
	require.Equal(t, toModelLabelSet(ls1), val[0].original, "first item should match ls1")
	require.Equal(t, toModelLabelSet(ls2), val[1].original, "second item should match ls2")
}

func TestCacheLRU(t *testing.T) {
	app := NewTestAppender()
	c, err := New(component.Options{
		Logger:        util.TestLogger(t),
		Registerer:    prometheus.NewRegistry(),
		OnStateChange: func(e component.Exports) {},
	}, Arguments{
		ForwardTo:      []pyroscope.Appendable{app},
		RelabelConfigs: []*alloy_relabel.Config{},
		MaxCacheSize:   2,
	})
	require.NoError(t, err)

	// Add profiles up to cache size
	labels1 := labels.FromStrings("env", "prod")
	labels2 := labels.FromStrings("env", "dev")
	labels3 := labels.FromStrings("env", "stage")

	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: labels1})
	require.NoError(t, err)
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: labels2})
	require.NoError(t, err)

	// Add one more to trigger eviction
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{Labels: labels3})
	require.NoError(t, err)

	// Verify size and that oldest entry was evicted
	require.Equal(t, 2, c.cache.Len())
	_, ok := c.cache.Get(toModelLabelSet(labels1).Fingerprint())
	require.False(t, ok, "oldest entry should have been evicted")
}

type TestAppender struct {
	mu       sync.Mutex
	profiles []*pyroscope.IncomingProfile
}

func NewTestAppender() *TestAppender {
	return &TestAppender{
		profiles: make([]*pyroscope.IncomingProfile, 0),
	}
}

// Appender implements pyroscope.Appendable
func (t *TestAppender) Appender() pyroscope.Appender {
	return t
}

// Append implements pyroscope.Appender
func (t *TestAppender) Append(_ context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
	return nil
}

// AppendIngest implements pyroscope.Appender
func (t *TestAppender) AppendIngest(_ context.Context, profile *pyroscope.IncomingProfile) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.profiles = append(t.profiles, profile)
	return nil
}

func (t *TestAppender) Profiles() []*pyroscope.IncomingProfile {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.profiles
}

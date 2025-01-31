package relabel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/pyroscope/api/model/labelset"
	"github.com/grafana/regexp"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
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

			tc, err := componenttest.NewControllerFromID(util.TestLogger(t), "pyroscope.relabel")
			require.NoError(t, err)

			go func() {
				err = tc.Run(componenttest.TestContext(t), Arguments{
					ForwardTo:      []pyroscope.Appendable{app},
					RelabelConfigs: tt.rules,
					MaxCacheSize:   10,
				})
				require.NoError(t, err)
			}()

			// Wait for the component to be ready
			require.NoError(t, tc.WaitExports(time.Second))
			time.Sleep(100 * time.Millisecond) // Give a little extra time for initialization

			profile := &pyroscope.IncomingProfile{
				Labels: tt.inputLabels,
			}

			// Get the actual component
			comp, err := tc.GetComponent()
			require.NoError(t, err)
			c := comp.(*Component)

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

			// Compare labels instead of URL
			gotProfile := app.Profiles()[0]
			require.Equal(t, tt.wantLabels, gotProfile.Labels)
		})
	}
}

func TestCache(t *testing.T) {
	app := NewTestAppender()

	c, err := New(component.Options{
		Logger:        util.TestAlloyLogger(t),
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

	// Initial profiles
	lsets := []labels.Labels{
		labels.FromStrings("env", "prod"),
		labels.FromStrings("env", "dev"),
		labels.FromStrings("env", "test"),
	}

	// Add initial entries and verify cache state
	for _, ls := range lsets {
		err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{
			Labels: ls,
		})
		require.NoError(t, err)
	}
	require.Equal(t, 3, c.cache.Len(), "cache should have 3 entries after initial profiles")

	// Test cache hit
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{
		Labels: lsets[0],
	})
	require.NoError(t, err)
	require.Equal(t, 3, c.cache.Len(), "cache length should not change after cache hit")

	// Test colliding entries
	ls1 := model.LabelSet{
		"A":        "K6sjsNNczPl",
		"__name__": "app.cpu",
	}
	ls2 := model.LabelSet{
		"A":        "cswpLMIZpwt",
		"__name__": "app.cpu",
	}
	hash := ls1.Fingerprint()
	t.Logf("Original fingerprint from ls1: %v", hash)
	require.Equal(t, ls1.Fingerprint(), ls2.Fingerprint(), "expected labelset fingerprints to collide")

	// Add colliding profiles
	for _, ls := range []model.LabelSet{ls1, ls2} {
		// Convert directly to labels.Labels as the component expects
		lbls := labels.FromStrings("A", string(ls["A"]), "__name__", string(ls["__name__"]))

		// Log intermediate fingerprint
		tmpLabelSet := make(model.LabelSet, lbls.Len())
		lbls.Range(func(l labels.Label) {
			tmpLabelSet[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		})
		t.Logf("Intermediate fingerprint: %v", tmpLabelSet.Fingerprint())

		err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{
			Labels: lbls,
		})
		require.NoError(t, err)
	}

	t.Logf("Final fingerprint for cache lookup: %v", hash)

	t.Logf("Cache length after adding colliding profiles: %d", c.cache.Len())
	t.Logf("Attempting to get cache key with fingerprint: %v", hash)
	for _, k := range c.cache.Keys() {
		t.Logf("Available cache key: %v", k)
	}

	// Verify cache state after collisions
	require.Equal(t, 4, c.cache.Len(), "cache should have 4 entries")
	val, ok := c.cache.Get(hash)
	require.True(t, ok, "colliding entry should be in cache")
	items := val.([]cacheItem)
	require.Len(t, items, 2, "should have both colliding items under same key")
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

func formatLabelsForURL(lbls labels.Labels) string {
	var pairs []string
	lbls.Range(func(l labels.Label) {
		pairs = append(pairs, fmt.Sprintf("%s=%s", l.Name, l.Value))
	})
	return strings.Join(pairs, ",")
}

func BenchmarkRelabelProcess(b *testing.B) {
	lbls := labels.FromStrings(
		"__name__", "test_metric",
		"env", "prod",
		"service", "api",
		"region", "us-east",
	)

	cfg := &relabel.Config{
		SourceLabels: []model.LabelName{"env"},
		TargetLabel:  "environment",
		Action:       "replace",
		Regex:        relabel.MustNewRegexp("(.+)"),
		Replacement:  "$1",
	}

	b.Run("Process", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			relabel.Process(lbls, cfg)
		}
	})

	b.Run("ProcessBuilder", func(b *testing.B) {
		builder := labels.NewBuilder(lbls)
		for i := 0; i < b.N; i++ {
			relabel.ProcessBuilder(builder, cfg)
		}
	})
}

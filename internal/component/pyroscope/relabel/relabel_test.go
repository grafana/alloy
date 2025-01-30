package relabel

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"sync"
	"testing"

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
	"github.com/stretchr/testify/require"
)

func TestRelabeling(t *testing.T) {
	tests := []struct {
		name        string
		rules       []*alloy_relabel.Config
		inputName   string
		wantName    string
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
			inputName:   "app.cpu{}",
			wantName:    "app.cpu{}",
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
			inputName:   "app.cpu{foo=hello}",
			wantName:    "app.cpu{bar=hello}",
			wantDropped: false,
		},
		{
			name: "drop profile with matching drop label",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "drop",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("dev")},
			}},
			inputName:   "app.cpu{env=dev,region=us-1}",
			wantDropped: true,
		},
		{
			name: "keep profile with matching label",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "keep",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("prod")},
			}},
			inputName:   "app.cpu{env=prod,region=us-1}",
			wantName:    "app.cpu{env=prod,region=us-1}",
			wantDropped: false,
		},
		{
			name: "drop profile not matching keep",
			rules: []*alloy_relabel.Config{{
				SourceLabels: []string{"env"},
				Action:       "keep",
				Regex:        alloy_relabel.Regexp{Regexp: regexp.MustCompile("prod")},
			}},
			inputName:   "app.cpu{env=dev,region=us-1}",
			wantDropped: true,
		},
		{
			name: "drop __name__ label should affect profile name",
			rules: []*alloy_relabel.Config{
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("__name__")},
				},
			},
			inputName:   "app.cpu{env=prod}",
			wantName:    "{env=prod}",
			wantDropped: false,
		},
		{
			name: "drop all labels preserves profile name",
			rules: []*alloy_relabel.Config{
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile("env|region")},
				},
			},
			inputName:   "app.cpu{env=prod,region=us-1}",
			wantName:    "app.cpu{}",
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
			inputName:   "app.cpu",
			wantName:    "app.cpu{}",
			wantDropped: false,
		},
		{
			name: "drop profile with no name",
			rules: []*alloy_relabel.Config{
				{
					Action: "labeldrop",
					Regex:  alloy_relabel.Regexp{Regexp: regexp.MustCompile(".*")},
				},
			},
			inputName:   "{env=prod,region=us-1}",
			wantName:    "",
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
			inputName:   "app.cpu{env=prod,region=us-1}",
			wantName:    "app.cpu{environment=prod,region=us-1,zone=zone-1}",
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
				URL: mustParseURL("http://localhost/ingest?name=" + tt.inputName),
			}

			// Get the actual component
			comp, err := tc.GetComponent()
			require.NoError(t, err)
			c := comp.(*Component)

			err = c.AppendIngest(context.Background(), profile)

			profiles := app.Profiles()

			if tt.wantDropped {
				require.Error(t, err)
				if errors.Is(err, labelset.ErrServiceNameIsRequired) {
					require.Empty(t, profiles, "profile should have been dropped")
					return
				}
				var dropErr *ProfileDroppedError
				require.ErrorAs(t, err, &dropErr)
				require.Equal(t, dropErr.Error(), err.Error())
				require.Empty(t, profiles, "profile should have been dropped")
				return
			}

			require.NoError(t, err)
			require.Len(t, profiles, 1)
			require.Equal(t, tt.wantName, profiles[0].URL.Query().Get("name"))
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
			URL: mustParseURL(fmt.Sprintf("http://localhost/ingest?name=app.cpu{%s}", formatLabelsForURL(ls))),
		})
		require.NoError(t, err)
	}
	require.Equal(t, 3, c.cache.Len(), "cache should have 3 entries after initial profiles")

	// Test cache hit
	err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{
		URL: mustParseURL(fmt.Sprintf("http://localhost/ingest?name=app.cpu{%s}", formatLabelsForURL(lsets[0]))),
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
		// lbls := labels.FromStrings("A", string(ls["A"]))
		lbls := labels.FromStrings("A", string(ls["A"]), "__name__", string(ls["__name__"]))

		// Log intermediate fingerprint
		tmpLabelSet := make(model.LabelSet, lbls.Len())
		lbls.Range(func(l labels.Label) {
			tmpLabelSet[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		})
		t.Logf("Intermediate fingerprint: %v", tmpLabelSet.Fingerprint())

		err = c.AppendIngest(context.Background(), &pyroscope.IncomingProfile{
			URL: mustParseURL(fmt.Sprintf("http://localhost/ingest?name=app.cpu{%s}", formatLabelsForURL(lbls))),
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

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func formatLabelsForURL(lbls labels.Labels) string {
	var pairs []string
	lbls.Range(func(l labels.Label) {
		pairs = append(pairs, fmt.Sprintf("%s=%s", l.Name, l.Value))
	})
	return strings.Join(pairs, ",")
}

package promsdprocessor

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/component/discovery"
)

func TestSyncGroups(t *testing.T) {
	tests := []struct {
		name        string
		jobToSync   string
		relabelCfgs map[string][]*relabel.Config
		targets     []model.LabelSet
		expected    map[string]discovery.Target
	}{
		{
			name:        "empty",
			jobToSync:   "",
			relabelCfgs: map[string][]*relabel.Config{},
			targets:     []model.LabelSet{},
			expected:    map[string]discovery.Target{},
		},
		{
			name:      "no relabeling",
			jobToSync: "job",
			relabelCfgs: map[string][]*relabel.Config{
				"job": {},
			},
			targets: []model.LabelSet{
				{
					"__address__": "127.0.0.1",
				},
			},
			expected: map[string]discovery.Target{
				"127.0.0.1": discovery.EmptyTarget,
			},
		},
		{
			name:      "strip port",
			jobToSync: "job",
			relabelCfgs: map[string][]*relabel.Config{
				"job": {},
			},
			targets: []model.LabelSet{
				{
					"__address__": "127.0.0.1:8888",
					"label":       "val",
				},
			},
			expected: map[string]discovery.Target{
				"127.0.0.1": discovery.NewTargetFromMap(map[string]string{
					"label": "val",
				}),
			},
		},
		{
			name:      "passthrough",
			jobToSync: "job",
			relabelCfgs: map[string][]*relabel.Config{
				"job": {},
			},
			targets: []model.LabelSet{
				{
					"__address__": "127.0.0.1",
					"label":       "val",
				},
			},
			expected: map[string]discovery.Target{
				"127.0.0.1": discovery.NewTargetFromMap(map[string]string{
					"label": "val",
				}),
			},
		},
		{
			name:      "ignore metadata",
			jobToSync: "job",
			relabelCfgs: map[string][]*relabel.Config{
				"job": {},
			},
			targets: []model.LabelSet{
				{
					"__address__": "127.0.0.1",
					"__ignore":    "ignore",
				},
			},
			expected: map[string]discovery.Target{
				"127.0.0.1": discovery.EmptyTarget,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := []*targetgroup.Group{
				{
					Targets: tc.targets,
				},
			}

			p := &promServiceDiscoProcessor{
				logger:         log.NewNopLogger(),
				relabelConfigs: tc.relabelCfgs,
			}

			hostLabels := make(map[string]discovery.Target)
			p.syncGroups(tc.jobToSync, groups, hostLabels)

			assert.Equal(t, tc.expected, hostLabels)
		})
	}
}

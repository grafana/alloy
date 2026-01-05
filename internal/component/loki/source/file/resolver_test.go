package file

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
)

func TestResolver(t *testing.T) {
	type testCase struct {
		name     string
		resolver resolver
		targets  []discovery.Target
		expected []resolvedTarget
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	tests := []testCase{
		{
			name:     "static resolver",
			resolver: newStaticResolver(),
			targets: []discovery.Target{
				discovery.NewTargetFromLabelSet(model.LabelSet{
					"__path__":     "some path",
					"__internal__": "internal",
					"label":        "label",
				}),
			},
			expected: []resolvedTarget{
				{
					Path: "some path",
					Labels: model.LabelSet{
						"label": "label",
					},
				},
			},
		},
		{
			name:     "glob resolver",
			resolver: newGlobResolver(log.NewNopLogger()),
			targets: []discovery.Target{
				discovery.NewTargetFromLabelSet(model.LabelSet{
					"__path__":     "./testdata/*.log",
					"__internal__": "internal",
					"label":        "label",
				}),
			},
			expected: []resolvedTarget{
				{
					Path: filepath.Join(dir, "/testdata/onelinelog.log"),
					Labels: model.LabelSet{
						"label": "label",
					},
				},
				{
					Path: filepath.Join(dir, "/testdata/short-access.log"),
					Labels: model.LabelSet{
						"label": "label",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			for target := range tt.resolver.Resolve(tt.targets) {
				require.Equal(t, tt.expected[i], target)
				i += 1
			}
		})
	}
}

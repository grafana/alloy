package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJoinWithTruncation(t *testing.T) {
	type args struct {
		str          []string
		sep          string
		maxLength    int
		abbreviation string
	}
	tests := []struct {
		name     string
		args     args
		expected string
	}{
		{
			name:     "empty slice",
			args:     args{str: []string{}, sep: ", ", maxLength: 0, abbreviation: "..."},
			expected: "",
		},
		{
			name:     "empty slice 2",
			args:     args{str: []string{}, sep: ", ", maxLength: 10, abbreviation: "..."},
			expected: "",
		},
		{
			name:     "smaller slice",
			args:     args{str: []string{"one", "two", "three"}, sep: ", ", maxLength: 10},
			expected: "one, two, three",
		},
		{
			name: "truncate slice",
			args: args{
				str:          []string{"one", "two", "three", "four", "five", "six"},
				sep:          ", ",
				maxLength:    4,
				abbreviation: "[...]",
			},
			expected: "one, two, three, [...], six",
		},
		{
			name: "truncate to 0",
			args: args{
				str:          []string{"one", "two", "three", "four", "five", "six"},
				sep:          ", ",
				maxLength:    0,
				abbreviation: "[...]",
			},
			expected: "",
		},
		{
			name: "truncate to 1",
			args: args{
				str:          []string{"one", "two", "three", "four", "five", "six"},
				sep:          ", ",
				maxLength:    1,
				abbreviation: "[...]",
			},
			expected: "one, [...]",
		},
		{
			name: "truncate to 2",
			args: args{
				str:          []string{"one", "two", "three", "four", "five", "six"},
				sep:          ", ",
				maxLength:    2,
				abbreviation: "[...]",
			},
			expected: "one, [...], six",
		},
		{
			name: "single element to 0",
			args: args{
				str:          []string{"one"},
				sep:          ", ",
				maxLength:    0,
				abbreviation: "[...]",
			},
			expected: "",
		},
		{
			name: "single element to 1",
			args: args{
				str:          []string{"one"},
				sep:          ", ",
				maxLength:    1,
				abbreviation: "[...]",
			},
			expected: "one",
		},
		{
			name: "single element to 2",
			args: args{
				str:          []string{"one"},
				sep:          ", ",
				maxLength:    2,
				abbreviation: "[...]",
			},
			expected: "one",
		},
		{
			name: "cluster peers example",
			args: args{
				str: []string{
					"grafana-agent-helm-15.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090",
					"grafana-agent-helm-6.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090",
					"grafana-agent-helm-16.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090",
					"grafana-agent-helm-2.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090",
				},
				sep:          ",",
				maxLength:    2,
				abbreviation: "...",
			},
			expected: "grafana-agent-helm-15.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090," +
				"...," +
				"grafana-agent-helm-2.grafana-agent-helm.grafana-agent.svc.cluster.local.:3090",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := JoinWithTruncation(tt.args.str, tt.args.sep, tt.args.maxLength, tt.args.abbreviation)
			require.Equal(t, tt.expected, actual)
		})
	}
}

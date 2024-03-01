package component_test

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/stretchr/testify/require"
)

func TestMergeHealth(t *testing.T) {
	var (
		jan1 = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
		jan2 = time.Date(2023, time.January, 2, 0, 0, 0, 0, time.UTC)
	)

	_ = jan2

	tt := []struct {
		name        string
		healths     []component.Health
		expectIndex int
	}{
		{
			name: "returns first health",
			healths: []component.Health{{
				Health:     component.HealthTypeHealthy,
				UpdateTime: jan1,
			}},
			expectIndex: 0,
		},
		{
			name: "exited > unhealthy",
			healths: []component.Health{{
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan1,
			}, {
				Health:     component.HealthTypeExited,
				UpdateTime: jan1,
			}},
			expectIndex: 1,
		},
		{
			name: "unhealthy > healthy",
			healths: []component.Health{{
				Health:     component.HealthTypeHealthy,
				UpdateTime: jan1,
			}, {
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan1,
			}},
			expectIndex: 1,
		},
		{
			name: "unknown > healthy",
			healths: []component.Health{{
				Health:     component.HealthTypeHealthy,
				UpdateTime: jan1,
			}, {
				Health:     component.HealthTypeUnknown,
				UpdateTime: jan1,
			}},
			expectIndex: 1,
		},
		{
			name: "newer timestamp",
			healths: []component.Health{{
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan1,
			}, {
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan2,
			}},
			expectIndex: 1,
		},
		{
			name: "use first found of matching health type and time",
			healths: []component.Health{{
				Health:     component.HealthTypeHealthy,
				UpdateTime: jan2,
			}, {
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan2,
			}, {
				Health:     component.HealthTypeUnhealthy,
				UpdateTime: jan2,
			}},
			expectIndex: 1,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.healths) == 0 {
				panic("Must have at least one health in test case")
			}

			expect := tc.healths[tc.expectIndex]
			actual := component.LeastHealthy(tc.healths[0], tc.healths[1:]...)

			require.Equal(t, expect, actual)
		})
	}
}

package harnessflags

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	cases := []struct {
		name      string
		values    Values
		wantErr   bool
		errSubstr string
	}{
		{
			name:   "defaults are valid",
			values: Values{Parallel: 1},
		},
		{
			name:      "keep-deps without keep-cluster errors",
			values:    Values{KeepDeps: true, Parallel: 1},
			wantErr:   true,
			errSubstr: "keep-deps requires",
		},
		{
			name:   "keep-deps with keep-cluster is allowed",
			values: Values{KeepDeps: true, KeepCluster: true, Parallel: 1},
		},
		{
			name:      "reuse-deps without reuse-cluster errors",
			values:    Values{ReuseDeps: true, Parallel: 1},
			wantErr:   true,
			errSubstr: "reuse-deps requires",
		},
		{
			name:   "reuse-deps with reuse-cluster is allowed",
			values: Values{ReuseDeps: true, ReuseCluster: "my-cluster", Parallel: 1},
		},
		{
			name:      "pull-policy without image errors",
			values:    Values{AlloyPullPolicy: "IfNotPresent", Parallel: 1},
			wantErr:   true,
			errSubstr: "pull-policy requires",
		},
		{
			name:   "pull-policy with image is allowed",
			values: Values{AlloyPullPolicy: "IfNotPresent", AlloyImage: "alloy:dev", Parallel: 1},
		},
		{
			name:   "image alone is allowed",
			values: Values{AlloyImage: "alloy:dev", Parallel: 1},
		},
		{
			name:      "parallel zero errors",
			values:    Values{Parallel: 0},
			wantErr:   true,
			errSubstr: "parallel must be >= 1",
		},
		{
			name:      "parallel negative errors",
			values:    Values{Parallel: -1},
			wantErr:   true,
			errSubstr: "parallel must be >= 1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.values)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errSubstr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr && tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("error %q missing expected substring %q", err.Error(), tc.errSubstr)
			}
		})
	}
}

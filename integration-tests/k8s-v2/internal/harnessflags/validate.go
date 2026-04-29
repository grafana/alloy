// Package harnessflags holds the pure flag-validation logic used by the
// k8s-v2 harness so the full matrix can be exercised in unit tests without
// booting the integration test harness.
package harnessflags

import "fmt"

// Values is a pure snapshot of the harness flags Validate inspects.
type Values struct {
	KeepDeps        bool
	KeepCluster     bool
	ReuseDeps       bool
	ReuseCluster    string
	AlloyImage      string
	AlloyPullPolicy string
	Parallel        int
}

// Validate reports an error if v is internally inconsistent (for example
// asking to reuse dependencies without reusing a cluster).
func Validate(v Values) error {
	if v.KeepDeps && !v.KeepCluster {
		return fmt.Errorf("k8s.v2.keep-deps requires k8s.v2.keep-cluster=true")
	}
	if v.ReuseDeps && v.ReuseCluster == "" {
		return fmt.Errorf("k8s.v2.reuse-deps requires k8s.v2.reuse-cluster")
	}
	if v.AlloyPullPolicy != "" && v.AlloyImage == "" {
		return fmt.Errorf("k8s.v2.alloy-image-pull-policy requires k8s.v2.alloy-image")
	}
	if v.Parallel < 1 {
		return fmt.Errorf("k8s.v2.parallel must be >= 1")
	}
	return nil
}

//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"strings"
	"testing"
)

func TestNewTestRuntime(t *testing.T) {
	rt, err := newTestRuntime("k8s-v2-Metrics_Mimir")
	if err != nil {
		t.Fatalf("new runtime failed: %v", err)
	}
	if !strings.HasPrefix(rt.testID, "metrics-mimir-") {
		t.Fatalf("unexpected test id prefix: %q", rt.testID)
	}
	if rt.namespace != rt.testID {
		t.Fatalf("expected namespace to match test id: namespace=%q testID=%q", rt.namespace, rt.testID)
	}
	if rt.release != rt.namespace {
		t.Fatalf("expected release to match namespace: release=%q namespace=%q", rt.release, rt.namespace)
	}
	if len(rt.namespace) > maxNamespaceLength {
		t.Fatalf("namespace exceeds max length: %d", len(rt.namespace))
	}
	if len(rt.release) > maxReleaseLength {
		t.Fatalf("release exceeds max length: %d", len(rt.release))
	}
}

func TestJoinNameTruncatesAndKeepsSuffix(t *testing.T) {
	base := strings.Repeat("a", 120)
	name := joinName(base, "deadbeef", maxNamespaceLength)
	if len(name) > maxNamespaceLength {
		t.Fatalf("name too long: %d", len(name))
	}
	if !strings.HasSuffix(name, "-deadbeef") {
		t.Fatalf("name must keep suffix, got %q", name)
	}
}

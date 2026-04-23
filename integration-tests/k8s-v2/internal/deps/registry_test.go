package deps

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *callRecorder) add(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *callRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.calls)
}

type fakeInstaller struct {
	name         string
	namespace    string
	installErr   error
	uninstallErr error
	installDelay time.Duration
	recorder     *callRecorder
}

func (f fakeInstaller) Name() string      { return f.name }
func (f fakeInstaller) Namespace() string { return f.namespace }

func (f fakeInstaller) Install(_ context.Context, _ Env) error {
	if f.installDelay > 0 {
		time.Sleep(f.installDelay)
	}
	f.recorder.add("install:" + f.name)
	return f.installErr
}

func (f fakeInstaller) Uninstall(_ context.Context, _ Env) error {
	f.recorder.add("uninstall:" + f.name)
	return f.uninstallErr
}

func TestRegistryValidateUnknown(t *testing.T) {
	r := NewRegistry(Env{}, fakeInstaller{name: "loki", recorder: &callRecorder{}})
	err := r.Validate([]string{"loki", "mimir"})
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestRegistryInstallOrder(t *testing.T) {
	recorder := &callRecorder{}
	r := NewRegistry(Env{},
		fakeInstaller{name: "a", recorder: recorder},
		fakeInstaller{name: "b", recorder: recorder},
	)
	if err := r.Install(context.Background(), "kubeconfig", []string{"a", "b"}); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	calls := recorder.snapshot()
	if len(calls) != 2 {
		t.Fatalf("unexpected number of calls: %v", calls)
	}
	if !slices.Contains(calls, "install:a") || !slices.Contains(calls, "install:b") {
		t.Fatalf("missing install calls: %v", calls)
	}
}

func TestRegistryInstallPartialFailureRollsBack(t *testing.T) {
	recorder := &callRecorder{}
	r := NewRegistry(Env{},
		fakeInstaller{name: "a", installDelay: 10 * time.Millisecond, recorder: recorder},
		fakeInstaller{name: "b", installErr: errors.New("boom"), recorder: recorder},
	)
	err := r.Install(context.Background(), "kubeconfig", []string{"a", "b"})
	if err == nil {
		t.Fatal("expected install failure")
	}
	calls := recorder.snapshot()
	if !slices.Contains(calls, "install:a") || !slices.Contains(calls, "install:b") {
		t.Fatalf("missing install calls: %v", calls)
	}
	if !slices.Contains(calls, "uninstall:a") {
		t.Fatalf("expected rollback uninstall:a call, got %v", calls)
	}
}

func TestRegistryNamespaceLookup(t *testing.T) {
	r := NewRegistry(Env{},
		fakeInstaller{name: "a", namespace: "ns-a", recorder: &callRecorder{}},
	)
	if got := r.Namespace("a"); got != "ns-a" {
		t.Fatalf("namespace for a: want ns-a, got %q", got)
	}
	if got := r.Namespace("missing"); got != "" {
		t.Fatalf("missing namespace: want empty, got %q", got)
	}
}

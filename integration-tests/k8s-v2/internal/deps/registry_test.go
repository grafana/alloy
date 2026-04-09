package deps

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type fakeInstaller struct {
	name         string
	installErr   error
	uninstallErr error
	calls        *[]string
}

func (f fakeInstaller) Name() string { return f.name }

func (f fakeInstaller) Install(_ context.Context, _ string) error {
	*f.calls = append(*f.calls, "install:"+f.name)
	return f.installErr
}

func (f fakeInstaller) Uninstall(_ context.Context, _ string) error {
	*f.calls = append(*f.calls, "uninstall:"+f.name)
	return f.uninstallErr
}

func TestRegistryValidateUnknown(t *testing.T) {
	r := NewRegistry(fakeInstaller{name: "loki", calls: new([]string)})
	err := r.Validate([]string{"loki", "mimir"})
	if err == nil {
		t.Fatal("expected unknown dependency error")
	}
}

func TestRegistryInstallOrder(t *testing.T) {
	var calls []string
	r := NewRegistry(
		fakeInstaller{name: "a", calls: &calls},
		fakeInstaller{name: "b", calls: &calls},
	)
	if err := r.Install(context.Background(), "kubeconfig", []string{"a", "b"}); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	want := []string{"install:a", "install:b"}
	if !slices.Equal(calls, want) {
		t.Fatalf("unexpected calls: got=%v want=%v", calls, want)
	}
}

func TestRegistryInstallPartialFailureRollsBack(t *testing.T) {
	var calls []string
	r := NewRegistry(
		fakeInstaller{name: "a", calls: &calls},
		fakeInstaller{name: "b", installErr: errors.New("boom"), calls: &calls},
	)
	err := r.Install(context.Background(), "kubeconfig", []string{"a", "b"})
	if err == nil {
		t.Fatal("expected install failure")
	}
	want := []string{"install:a", "install:b", "uninstall:a"}
	if !slices.Equal(calls, want) {
		t.Fatalf("unexpected calls: got=%v want=%v", calls, want)
	}
}

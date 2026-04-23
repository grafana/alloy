package deps

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/backendspec"
)

// manifestInstaller installs and uninstalls a manifest-defined backend
// described by a backendspec.Spec. All manifest-based installers in the
// harness share this single implementation; only the Spec differs.
type manifestInstaller struct {
	spec backendspec.Spec
}

// NewManifestInstaller returns an Installer for a manifest-based backend
// described by spec. The manifest content is loaded from embedded files
// in the manifests/ directory by name (spec.ManifestFile).
func NewManifestInstaller(spec backendspec.Spec) Installer {
	return manifestInstaller{spec: spec}
}

// NewLokiInstaller is a convenience wrapper for the default Loki backend.
func NewLokiInstaller() Installer {
	return NewManifestInstaller(backendspec.Loki)
}

// NewMimirInstaller is a convenience wrapper for the default Mimir backend.
func NewMimirInstaller() Installer {
	return NewManifestInstaller(backendspec.Mimir)
}

func (m manifestInstaller) Name() string      { return m.spec.Name }
func (m manifestInstaller) Namespace() string { return m.spec.Namespace }

func (m manifestInstaller) Install(ctx context.Context, env Env) error {
	manifest, err := readManifest(m.spec.ManifestFile)
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, env, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, env, m.spec.Namespace, m.spec.Deployment); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(ctx, env, m.spec.Namespace, m.spec.Service, m.spec.Port, m.spec.ReadinessPath); err != nil {
		return fmt.Errorf("dependency=%s: %w", m.spec.Name, err)
	}
	return nil
}

func (m manifestInstaller) Uninstall(ctx context.Context, env Env) error {
	manifest, err := readManifest(m.spec.ManifestFile)
	if err != nil {
		return err
	}
	return deleteManifest(ctx, env, manifest)
}

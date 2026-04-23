package deps

import (
	"context"
	"fmt"
)

type mimirInstaller struct{}

func NewMimirInstaller() Installer {
	return mimirInstaller{}
}

func (m mimirInstaller) Name() string {
	return "mimir"
}

func (m mimirInstaller) Namespace() string {
	return MimirNamespace
}

func (m mimirInstaller) Install(ctx context.Context, env Env) error {
	manifest, err := readManifest("mimir.yaml")
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, env, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, env, MimirNamespace, "mimir"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(ctx, env, MimirNamespace, "mimir", 9009, "/ready"); err != nil {
		return fmt.Errorf("dependency=%s: %w", m.Name(), err)
	}
	return nil
}

func (m mimirInstaller) Uninstall(ctx context.Context, env Env) error {
	manifest, err := readManifest("mimir.yaml")
	if err != nil {
		return err
	}
	return deleteManifest(ctx, env, manifest)
}

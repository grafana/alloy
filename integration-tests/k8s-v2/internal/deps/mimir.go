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

func (m mimirInstaller) Install(ctx context.Context, kubeconfig string) error {
	manifest, err := readManifest("mimir.yaml")
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, kubeconfig, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, kubeconfig, mimirNamespace, "mimir"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(
		ctx,
		kubeconfig,
		mimirNamespace,
		"mimir",
		39009,
		9009,
		"http://127.0.0.1:39009/ready",
	); err != nil {
		return fmt.Errorf("dependency=%s: %w", m.Name(), err)
	}
	return nil
}

func (m mimirInstaller) Uninstall(ctx context.Context, kubeconfig string) error {
	manifest, err := readManifest("mimir.yaml")
	if err != nil {
		return err
	}
	return deleteManifest(ctx, kubeconfig, manifest)
}

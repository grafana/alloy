package deps

import (
	"context"
	"fmt"
)

type lokiInstaller struct{}

func NewLokiInstaller() Installer {
	return lokiInstaller{}
}

func (l lokiInstaller) Name() string {
	return "loki"
}

func (l lokiInstaller) Install(ctx context.Context, kubeconfig string) error {
	manifest, err := readManifest("loki.yaml")
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, kubeconfig, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, kubeconfig, lokiNamespace, "loki"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(
		ctx,
		kubeconfig,
		lokiNamespace,
		"loki",
		33100,
		3100,
		"http://127.0.0.1:33100/ready",
	); err != nil {
		return fmt.Errorf("dependency=%s: %w", l.Name(), err)
	}
	return nil
}

func (l lokiInstaller) Uninstall(ctx context.Context, kubeconfig string) error {
	manifest, err := readManifest("loki.yaml")
	if err != nil {
		return err
	}
	return deleteManifest(ctx, kubeconfig, manifest)
}

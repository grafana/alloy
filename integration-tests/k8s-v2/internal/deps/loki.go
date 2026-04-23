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

func (l lokiInstaller) Namespace() string {
	return LokiNamespace
}

func (l lokiInstaller) Install(ctx context.Context, env Env) error {
	manifest, err := readManifest("loki.yaml")
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, env, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, env, LokiNamespace, "loki"); err != nil {
		return err
	}
	if err := checkServiceReadyEndpoint(ctx, env, LokiNamespace, "loki", 3100, "/ready"); err != nil {
		return fmt.Errorf("dependency=%s: %w", l.Name(), err)
	}
	return nil
}

func (l lokiInstaller) Uninstall(ctx context.Context, env Env) error {
	manifest, err := readManifest("loki.yaml")
	if err != nil {
		return err
	}
	return deleteManifest(ctx, env, manifest)
}

package deps

import (
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

// ensureKindImage pulls image if missing and loads it into the runner's
// kind cluster. Use it for deps pulling a pinned upstream image (e.g.
// prom/blackbox-exporter); images built from this repo are pre-loaded
// by the runner.
func ensureKindImage(image string) error {
	if image == "" {
		return fmt.Errorf("image is required")
	}
	cluster := harness.KindClusterName()
	if cluster == "" {
		return fmt.Errorf("kind cluster name not set; ensure the test runner exported ALLOY_TESTS_KIND_CLUSTER")
	}

	if err := ensureLocalImage(image); err != nil {
		return err
	}
	return util.Step(fmt.Sprintf("kind load %s", image), func() error {
		return harness.RunCommand("kind", "load", "docker-image", image, "--name", cluster)
	})
}

// ensureLocalImage pulls image only if it's not already in the local daemon.
func ensureLocalImage(image string) error {
	if err := harness.RunCommandQuiet("docker", "image", "inspect", image); err == nil {
		return nil
	}
	return util.Step(fmt.Sprintf("docker pull %s", image), func() error {
		return harness.RunCommand("docker", "pull", image)
	})
}

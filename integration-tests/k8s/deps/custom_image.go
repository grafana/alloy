package deps

import (
	"fmt"
	"path/filepath"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

// CustomImageOptions describes docker image to build and load into the runner's kind cluster before the test runs.
type CustomImageOptions struct {
	// Tag is the image tag.
	Tag string

	// ContextPath is the docker build context, resolved relative to the
	// test package's working directory.
	ContextPath string

	// DockerfilePath optionally overrides the default <ContextPath>/Dockerfile.
	DockerfilePath string
}

// CustomImage builds a docker image loads it into the kind cluster.
type CustomImage struct {
	opts CustomImageOptions
}

func NewCustomImage(opts CustomImageOptions) *CustomImage {
	return &CustomImage{opts: opts}
}

// Name includes the tag so multiple instances are distinguishable in logs.
func (c *CustomImage) Name() string {
	if c.opts.Tag == "" {
		return "custom-image"
	}
	return "custom-image (" + c.opts.Tag + ")"
}

func (c *CustomImage) Install(_ *harness.TestContext) error {
	if c.opts.Tag == "" {
		return fmt.Errorf("custom image tag is required")
	}
	if c.opts.ContextPath == "" {
		return fmt.Errorf("custom image context path is required")
	}

	absContext, err := filepath.Abs(c.opts.ContextPath)
	if err != nil {
		return fmt.Errorf("resolve custom image context: %w", err)
	}

	dockerfile := c.opts.DockerfilePath
	if dockerfile == "" {
		dockerfile = filepath.Join(absContext, "Dockerfile")
	} else {
		dockerfile, err = filepath.Abs(dockerfile)
		if err != nil {
			return fmt.Errorf("resolve custom image dockerfile: %w", err)
		}
	}

	if err := util.Step(fmt.Sprintf("docker build %s", c.opts.Tag), func() error {
		return harness.RunCommand("docker", "build", "-t", c.opts.Tag, "-f", dockerfile, absContext)
	}); err != nil {
		return err
	}

	cluster := harness.KindClusterName()
	if cluster == "" {
		return fmt.Errorf("kind cluster name not set; ensure the test runner exported ALLOY_TESTS_KIND_CLUSTER")
	}
	return util.Step(fmt.Sprintf("kind load %s", c.opts.Tag), func() error {
		return harness.RunCommand("kind", "load", "docker-image", c.opts.Tag, "--name", cluster)
	})
}

func (c *CustomImage) Cleanup() {}

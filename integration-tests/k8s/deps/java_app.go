package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/java-app.yaml
var javaAppManifest string

const (
	javaAppImage       = "eclipse-temurin:21.0.8_9-jdk-noble"
	javaAppClientImage = "busybox:1.36"
	javaAppSelector    = "component=java-app"
)

// JavaApp installs a JVM serving HTTP on port 8080 as the "java-app" Service,
// and a client that sends it a request every second.
type JavaApp struct {
	opts      JavaAppOptions
	installed bool
}

type JavaAppOptions struct {
	Namespace string
}

func NewJavaApp(opts JavaAppOptions) *JavaApp {
	return &JavaApp{opts: opts}
}

func (j *JavaApp) Name() string { return "java-app" }

func (j *JavaApp) Install(_ *harness.TestContext) error {
	if j.opts.Namespace == "" {
		return fmt.Errorf("java-app namespace is required")
	}
	for _, image := range []string{javaAppImage, javaAppClientImage} {
		if err := ensureKindImage(image); err != nil {
			return err
		}
	}
	if err := util.Step("apply java-app manifest", func() error {
		return harness.ApplyManifest(j.opts.Namespace, javaAppManifest)
	}); err != nil {
		return err
	}
	j.installed = true
	return util.Step("wait for java-app pods ready", func() error {
		return harness.WaitForReady(j.opts.Namespace, javaAppSelector)
	})
}

func (j *JavaApp) Cleanup() {
	if !j.installed {
		return
	}
	_ = harness.DeleteManifest(j.opts.Namespace, javaAppManifest)
}

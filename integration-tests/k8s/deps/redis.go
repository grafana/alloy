package deps

import (
	_ "embed"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/redis.yaml
var redisManifest string

const redisSelector = "app=redis"

var _ harness.Dependency = (*Redis)(nil)

// Redis installs a Redis instance as a scrape target for the
// prometheus.exporter.redis component, seeded with keys in two databases so
// the keyspace metrics are populated.
type Redis struct {
	opts      RedisOptions
	installed bool
}

type RedisOptions struct {
	Namespace string
}

func NewRedis(opts RedisOptions) *Redis {
	return &Redis{opts: opts}
}

func (r *Redis) Name() string { return "redis" }

func (r *Redis) Install(_ *harness.TestContext) error {
	if r.opts.Namespace == "" {
		return fmt.Errorf("redis namespace is required")
	}
	if err := util.Step("apply redis manifest", func() error {
		return harness.ApplyManifest(r.opts.Namespace, redisManifest)
	}); err != nil {
		return err
	}
	r.installed = true

	if err := util.Step("wait for redis pod ready", func() error {
		return harness.WaitForReady(r.opts.Namespace, redisSelector)
	}); err != nil {
		return err
	}

	return util.Step("wait for redis seed job", func() error {
		return harness.RunCommand("kubectl",
			"--namespace", r.opts.Namespace,
			"wait", "--for=condition=complete", "job/redis-seed",
			"--timeout=2m",
		)
	})
}

func (r *Redis) Cleanup() {
	if !r.installed {
		return
	}
	_ = harness.DeleteManifest(r.opts.Namespace, redisManifest)
}

package harness

import (
	"context"
	"os"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const diagTimeout = 20 * time.Second

type diagnosticHook struct {
	name string
	fn   func(context.Context) error
}

func collectFailureDiagnostics(ctx *TestContext) {
	util.Logf("collecting failure diagnostics for test %q", ctx.name)
	for _, hook := range ctx.diagnosticHooks {
		hookCtx, cancel := context.WithTimeout(context.Background(), diagTimeout)
		start := time.Now()
		err := hook.fn(hookCtx)
		cancel()
		if err != nil {
			util.Logf("diagnostics hook failed name=%q time=%s err=%v", hook.name, time.Since(start).Round(time.Millisecond), err)
			continue
		}
		util.Logf("diagnostics hook done name=%q time=%s", hook.name, time.Since(start).Round(time.Millisecond))
	}
	if ctx.pkgPath != "" {
		util.Logf("repro: make integration-test-k8s RUN_ARGS='--package ./%s'", ctx.pkgPath)
	}
	util.Logf("kubeconfig: %s", os.Getenv(KubeconfigEnv))
}

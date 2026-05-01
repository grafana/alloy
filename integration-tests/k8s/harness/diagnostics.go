package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const diagTimeout = 20 * time.Second

type diagnosticHook struct {
	name string
	fn   func(context.Context) error
}

func collectFailureDiagnostics(ctx *TestContext) {
	fmt.Printf("[k8s-itest] collecting failure diagnostics namespace=%s\n", ctx.namespace)
	for _, hook := range ctx.diagnosticHooks {
		hookCtx, cancel := context.WithTimeout(context.Background(), diagTimeout)
		start := time.Now()
		err := hook.fn(hookCtx)
		cancel()
		if err != nil {
			fmt.Printf("[k8s-itest] diagnostics hook failed name=%q time=%s err=%v\n", hook.name, time.Since(start).Round(time.Millisecond), err)
			continue
		}
		fmt.Printf("[k8s-itest] diagnostics hook done name=%q time=%s\n", hook.name, time.Since(start).Round(time.Millisecond))
	}
	fmt.Printf("[k8s-itest] repro: make integration-test-k8s RUN_ARGS='--package ./integration-tests/k8s/tests/%s'\n", ctx.name)
	fmt.Printf("[k8s-itest] kubeconfig: %s\n", os.Getenv(kubeconfigEnv))
}

func namespaceDiagnosticsHook(namespace string) func(context.Context) error {
	return func(c context.Context) error {
		return runDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", namespace, "get", "pods", "-o", "wide"},
			{"kubectl", "--namespace", namespace, "describe", "pods"},
		})
	}
}

func alloyDiagnosticsHook(namespace string) func(context.Context) error {
	return func(c context.Context) error {
		return runDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", namespace, "logs", "-l", "app.kubernetes.io/name=alloy", "--all-containers=true", "--tail", "200"},
		})
	}
}

func runDiagnosticCommands(c context.Context, commands [][]string) error {
	var errs []string
	for _, args := range commands {
		if len(args) == 0 {
			continue
		}
		if err := runDiagnosticCommand(c, args[0], args[1:]...); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func runDiagnosticCommand(c context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(c, name, args...)
	cmd.Env = commandEnv()
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if out.Len() > 0 {
		fmt.Printf("%s", out.String())
	}
	if err == nil {
		return nil
	}
	if c.Err() != nil {
		return fmt.Errorf("%s %v timed out: %w", name, args, c.Err())
	}
	return fmt.Errorf("%s %v failed: %w", name, args, err)
}

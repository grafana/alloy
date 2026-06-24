package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// RunCommand runs name with args, inheriting stdout/stderr; CommandEnv pins KUBECONFIG.
func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = CommandEnv()
	return cmd.Run()
}

// RunCommandQuiet is RunCommand with stdout/stderr discarded; use it when
// only the exit code matters (e.g. `docker image inspect`).
func RunCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Env = CommandEnv()
	return cmd.Run()
}

// RunCommandStdin is RunCommand with stdin piped from the given string.
// Useful for `kubectl apply -f -` style invocations.
func RunCommandStdin(stdin, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = CommandEnv()
	return cmd.Run()
}

// runDiagnosticCommand runs name under ctx and prints combined output.
// Used by failure-diagnostics hooks; surfaces ctx timeouts in errors.
func runDiagnosticCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = CommandEnv()
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
	if ctx.Err() != nil {
		return fmt.Errorf("%s %v timed out: %w", name, args, ctx.Err())
	}
	return fmt.Errorf("%s %v failed: %w", name, args, err)
}

// RunDiagnosticCommands runs each command, joining errors so one failure
// doesn't skip the rest. Returns a single joined error (or nil).
func RunDiagnosticCommands(ctx context.Context, commands [][]string) error {
	var errs []string
	for _, args := range commands {
		if len(args) == 0 {
			continue
		}
		if err := runDiagnosticCommand(ctx, args[0], args[1:]...); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

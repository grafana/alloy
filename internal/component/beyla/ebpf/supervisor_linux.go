//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup" //nolint:depguard

	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
	http_service "github.com/grafana/alloy/internal/service/http"
)

func (c *Component) handleSubprocessStart(ctx context.Context) {
	c.stopSubprocess()

	restartCount := c.subprocess.RecordStart()

	if restartCount > 0 {
		c.opts.Logger.Info("restarting Beyla subprocess", "restart_count", restartCount)
	} else {
		c.opts.Logger.Info("starting Beyla subprocess")
	}

	newCtx, cancelFunc := context.WithCancel(ctx)

	if err := c.setupSubprocess(); err != nil {
		cancelFunc()
		return
	}

	g, launchCtx := errgroup.WithContext(newCtx)
	g.Go(func() error { return c.runSubprocess(launchCtx) })
	g.Go(func() error { return c.watchdogLoop(launchCtx) })

	go c.monitorSubprocess(newCtx, g)

	c.subprocessCancel = cancelFunc
	c.subprocessGroup = g
}

// monitorSubprocess waits for the subprocess goroutines to exit and, on a crash
// (as opposed to an intentional stop or shutdown), reports the error to the Run
// loop, which owns restart scheduling. The ctx here is the subprocess's own
// cancelable context, so an intentional stop cancels it and is not seen as a crash.
func (c *Component) monitorSubprocess(ctx context.Context, g *errgroup.Group) {
	err := g.Wait()

	if ctx.Err() != nil || err == nil {
		return
	}

	select {
	case c.subprocessExit <- err:
	case <-ctx.Done():
	}
}

func (c *Component) handleSubprocessExit(err error) {
	c.opts.Logger.Error("Beyla subprocess crashed, scheduling restart", "err", err)
	c.health.SetUnhealthy(err)
	c.scheduleRestart()
}

func (c *Component) setupSubprocess() error {
	if err := c.startOTLPReceiver(); err != nil {
		c.opts.Logger.Error("failed to start OTLP receiver", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	port, err := findFreePort()

	if err != nil {
		c.opts.Logger.Error("failed to allocate port", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetListen(port, fmt.Sprintf("http://127.0.0.1:%d", port))

	if err := c.allocateProfilePort(); err != nil {
		c.opts.Logger.Error("failed to allocate Beyla profile port", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetHealthAddr(abstractSocketAddr("health", c.opts.ID))

	configPath, cleanupConfig, err := c.writeConfigFile()

	if err != nil {
		c.opts.Logger.Error("failed to write config", "err", err)
		c.health.SetUnhealthy(err)
		c.cleanup()
		c.scheduleRestart()
		return err
	}

	c.subprocess.SetConfig(configPath, cleanupConfig)

	return nil
}

func (c *Component) allocateProfilePort() error {
	data, err := c.opts.GetServiceData(http_service.ServiceName)

	if err != nil {
		return fmt.Errorf("failed to get HTTP service data: %w", err)
	}

	if !data.(http_service.Data).EnablePProf {
		return nil
	}

	port, err := findFreePort()

	if err != nil {
		return err
	}

	c.subprocess.SetProfilePort(port)

	return nil
}

func (c *Component) scheduleRestart() {
	backoff, count, ok := c.subprocess.NextBackoff()

	if !ok {
		c.opts.Logger.Error("Beyla subprocess exceeded maximum restart attempts, giving up", "max_restarts", subprocess.MaxRestarts)
		return
	}

	c.opts.Logger.Info("scheduling subprocess restart", "backoff", backoff, "restart_count", count)
	c.restartTimer.Reset(backoff)
}

func (c *Component) stopSubprocess() {
	if c.subprocessCancel == nil {
		return
	}

	c.subprocessCancel()
	c.opts.Logger.Info("waiting for Beyla subprocess to terminate")

	if err := c.subprocessGroup.Wait(); err != nil {
		c.opts.Logger.Error("Beyla subprocess terminated with error", "err", err)
	}

	c.cleanup()

	c.subprocessCancel = nil
	c.subprocessGroup = nil
}

func (c *Component) cleanup() {
	c.stopOTLPReceiver()
	c.otlpReceiverAddr = ""
	c.subprocess.Reset()
}

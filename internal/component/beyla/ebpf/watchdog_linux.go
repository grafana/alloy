//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

func (c *Component) watchdogLoop(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	client := &http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", c.subprocess.HealthAddr())
			},
		},
	}
	defer client.CloseIdleConnections()

	// Beyla loads eBPF programs before opening its Prometheus port (~15-20s)
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(20 * time.Second):
	}

	const (
		successesNeededToResetBackoff = 3
		failuresBeforeKill            = 5
	)
	var consecutiveSuccesses, consecutiveFailures int

	handleProbe := func(probeErr error) error {
		if probeErr != nil {
			consecutiveFailures++

			c.opts.Logger.Warn("subprocess probe failed", "err", probeErr, "consecutive_failures", consecutiveFailures)
			c.health.SetUnhealthy(probeErr)

			consecutiveSuccesses = 0

			if consecutiveFailures >= failuresBeforeKill {
				return fmt.Errorf("subprocess unresponsive after %d consecutive probe failures", consecutiveFailures)
			}
			return nil
		}

		consecutiveFailures = 0
		c.subprocess.SetReady(true)
		c.health.SetHealthy()
		consecutiveSuccesses++

		if consecutiveSuccesses >= successesNeededToResetBackoff {
			if c.subprocess.ResetBackoffIfElevated() {
				c.opts.Logger.Debug("resetting restart backoff after successful probes")
			}

			consecutiveSuccesses = 0
		}

		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := handleProbe(c.probeSubprocess(ctx, client)); err != nil {
				return err
			}
		}
	}
}

func (c *Component) probeSubprocess(ctx context.Context, client *http.Client) error {
	if c.subprocess.HealthAddr() == "" {
		return fmt.Errorf("subprocess not started")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// "beyla" is a placeholder, the client's DialContext ignores it and
	// dials the abstract unix socket
	req, err := http.NewRequestWithContext(ctx, "GET", "http://beyla/healthz", nil)

	if err != nil {
		return err
	}

	resp, err := client.Do(req)

	if err != nil {
		return fmt.Errorf("subprocess not responding: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subprocess returned status %d", resp.StatusCode)
	}

	return nil
}

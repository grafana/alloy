package container

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
)

const perlImage = "perl:5.44.0-slim@sha256:a0c68e9afd12dc9a10c0693f1dacac08e93d41a9e2f43aefc3a209c53e5147d2"

// StartPerlContainer runs script with perl and returns the host PID of the perl process.
func StartPerlContainer(t *testing.T, ctx context.Context, l *slog.Logger, script string) (testcontainers.Container, int) {
	req := testcontainers.ContainerRequest{
		Image: perlImage,
		Cmd:   []string{"perl", "-e", script},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PidMode = "host"
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           slog.NewLogLogger(l.Handler(), slog.LevelDebug),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := testcontainers.TerminateContainer(c)
		require.NoError(t, err)
	})

	inspected, err := c.Inspect(t.Context())
	require.NoError(t, err)

	return c, inspected.State.Pid
}

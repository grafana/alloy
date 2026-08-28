package container

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartPyroscopeContainer(t *testing.T, ctx context.Context, l *slog.Logger) (testcontainers.Container, string) {
	req := testcontainers.ContainerRequest{
		Image:        "grafana/pyroscope:latest",
		Cmd:          []string{"--ingester.min-ready-duration=0s"},
		ExposedPorts: []string{"4040/tcp"},
		WaitingFor:   wait.ForHTTP("/ready").WithPort("4040/tcp"),
		Env: map[string]string{
			"PYROSCOPE_LOG_LEVEL": "debug",
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

	mappedPort, err := c.MappedPort(ctx, "4040/tcp")
	require.NoError(t, err)

	host, err := c.Host(ctx)
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	return c, endpoint
}

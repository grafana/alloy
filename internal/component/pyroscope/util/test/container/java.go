package container

import (
	"context"
	"fmt"
	stdlog "log"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func StartJavaApplicationContainer(t *testing.T, ctx context.Context, l log.Logger) (testcontainers.Container, string, int) {
	req := testcontainers.ContainerRequest{
		Image:        "springcommunity/spring-framework-petclinic:latest",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor:   wait.ForHTTP("/").WithPort("8080/tcp").WithStartupTimeout(3 * time.Minute),
		Env: map[string]string{
			"JAVA_OPTS": "-Xmx512m -Xms256m",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PidMode = "host"
		},
	}

	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
		Logger:           stdlog.New(log.NewStdlibAdapter(l), "", 0),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		err := testcontainers.TerminateContainer(c)
		require.NoError(t, err)
	})

	mappedPort, err := c.MappedPort(ctx, nat.Port("8080/tcp"))
	require.NoError(t, err)

	host, err := c.Host(ctx)
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	inspected, err := c.Inspect(t.Context())
	require.NoError(t, err)

	return c, endpoint, inspected.State.Pid
}

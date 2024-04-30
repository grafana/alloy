//go:build !nodocker

package secrets_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/docker/go-connections/nat"
	"github.com/grafana/alloy/internal/alloy/componenttest"
	"github.com/grafana/alloy/internal/component"
	aws_common_config "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func Test_GetSecrets(t *testing.T) {
	var (
		ctx = componenttest.TestContext(t)
		ep  = runTestLocalSecretManager(t)
		l   = util.TestLogger(t)
	)

	cfg := fmt.Sprintf(`
	client {
		endpoint = "%s"
		key = "test"
		secret = "test"
	}
	id = "foo"
`, ep)

	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	ctrl, err := componenttest.NewControllerFromID(l, "remote.aws.secrets_manager")
	require.NoError(t, err)

	go func() {
		require.NoError(t, ctrl.Run(ctx, args))
	}()

	require.NoError(t, ctrl.WaitRunning(time.Minute))
	require.NoError(t, ctrl.WaitExports(time.Minute))

	var (
		expectExports = Exports{
			Data: map[string]alloytypes.Secret{
				"foo": alloytypes.Secret("bar"),
			},
		}
		actualExports = ctrl.Exports().(Exports)
	)
	require.Equal(t, expectExports, actualExports)

	innerComponent := ctrl.GetInnerComponent().(*Component)
	require.Equal(t, innerComponent.CurrentHealth().Health, component.HealthTypeHealthy)
}

func runTestLocalSecretManager(t *testing.T) string {
	ctx := componenttest.TestContext(t)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "localstack/localstack:3.4.0",
			ExposedPorts: []string{"4566/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("4566/tcp"),
			),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx))
	})

	ep, err := container.PortEndpoint(ctx, nat.Port("4566/tcp"), "http")
	require.NoError(t, err)

	// Create a secret with Localstack's SecretManager client
	awsCfg, err := aws_common_config.GenerateAWSConfig(aws_common_config.Client{
		AccessKey: "test",
		Secret:    alloytypes.Secret("test"),
		Endpoint:  ep,
		Region:    "us-east-1",
	})
	require.NoError(t, err)

	svc := secretsmanager.NewFromConfig(*awsCfg)

	secretName := "foo"
	secretString := "bar"
	result, err := svc.CreateSecret(context.TODO(), &secretsmanager.CreateSecretInput{
		Name:         &secretName,
		SecretString: &secretString,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	return ep
}

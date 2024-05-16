//go:build !nodocker

package secrets_manager

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/docker/go-connections/nat"
	"github.com/grafana/alloy/internal/component"
	aws_common_config "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func Test_GetSecrets(t *testing.T) {
	var (
		secretId = "foo"
		ctx      = componenttest.TestContext(t)
		ep, _    = runTestLocalSecretManager(t, secretId)
		l        = util.TestLogger(t)
	)

	cfg := fmt.Sprintf(`
	client {
		endpoint = "%s"
		key = "test"
		secret = "test"
	}
	id = "%s"
`, ep, secretId)

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
				secretId: alloytypes.Secret("bar"),
			},
		}
		actualExports = ctrl.Exports().(Exports)
	)
	require.Equal(t, expectExports, actualExports)

	innerComponent := ctrl.GetInnerComponent().(*Component)
	require.Equal(t, innerComponent.CurrentHealth().Health, component.HealthTypeHealthy)
}

func Test_PollSecrets(t *testing.T) {
	var (
		secretId   = "foo_poll"
		ctx        = componenttest.TestContext(t)
		ep, client = runTestLocalSecretManager(t, secretId)
		l          = util.TestLogger(t)
	)

	cfg := fmt.Sprintf(`
	client {
		endpoint = "%s"
		key = "test"
		secret = "test"
	}

	poll_frequency = "1s"

	id = "%s"
`, ep, secretId)

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
				secretId: alloytypes.Secret("bar"),
			},
		}
		actualExports = ctrl.Exports().(Exports)
	)
	require.Equal(t, expectExports, actualExports)

	// Updated the secret to something else
	updatedSecretString := "bar_poll"
	result, err := client.UpdateSecret(context.TODO(), &secretsmanager.UpdateSecretInput{
		SecretId:     &secretId,
		SecretString: &updatedSecretString,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.NoError(t, ctrl.WaitExports(time.Minute))

	expectExports = Exports{
		Data: map[string]alloytypes.Secret{
			secretId: alloytypes.Secret(updatedSecretString),
		},
	}
	actualExports = ctrl.Exports().(Exports)
	require.Equal(t, expectExports, actualExports)

}

func runTestLocalSecretManager(t *testing.T, secretId string) (string, *secretsmanager.Client) {
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

	secretString := "bar"
	result, err := svc.CreateSecret(context.TODO(), &secretsmanager.CreateSecretInput{
		Name:         &secretId,
		SecretString: &secretString,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	return ep, svc
}

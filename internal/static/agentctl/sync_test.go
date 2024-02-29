package agentctl

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/agent/internal/static/metrics/cluster/configapi"
	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/stretchr/testify/require"
)

func TestConfigSync_EmptyStore(t *testing.T) {
	cli := &mockFuncPromClient{}
	cli.ListConfigsFunc = func(_ context.Context) (*configapi.ListConfigurationsResponse, error) {
		return &configapi.ListConfigurationsResponse{}, nil
	}

	var putConfigs []string
	cli.PutConfigurationFunc = func(_ context.Context, name string, _ *instance.Config) error {
		putConfigs = append(putConfigs, name)
		return nil
	}

	err := ConfigSync(nil, cli, "./testdata", false)
	require.NoError(t, err)

	expect := []string{
		"agent-1",
		"agent-2",
		"agent-3",
	}
	require.Equal(t, expect, putConfigs)
}

func TestConfigSync_PrepopulatedStore(t *testing.T) {
	cli := &mockFuncPromClient{}
	cli.ListConfigsFunc = func(_ context.Context) (*configapi.ListConfigurationsResponse, error) {
		return &configapi.ListConfigurationsResponse{
			Configs: []string{"delete-a", "agent-1", "delete-b", "delete-c"},
		}, nil
	}

	var putConfigs []string
	cli.PutConfigurationFunc = func(_ context.Context, name string, _ *instance.Config) error {
		putConfigs = append(putConfigs, name)
		return nil
	}

	var deletedConfigs []string
	cli.DeleteConfigurationFunc = func(_ context.Context, name string) error {
		deletedConfigs = append(deletedConfigs, name)
		return nil
	}

	err := ConfigSync(nil, cli, "./testdata", false)
	require.NoError(t, err)

	expectUpdated := []string{
		"agent-1",
		"agent-2",
		"agent-3",
	}
	require.Equal(t, expectUpdated, putConfigs)

	expectDeleted := []string{
		"delete-a",
		"delete-b",
		"delete-c",
	}
	require.Equal(t, expectDeleted, deletedConfigs)
}

func TestConfigSync_DryRun(t *testing.T) {
	cli := &mockFuncPromClient{}
	cli.ListConfigsFunc = func(_ context.Context) (*configapi.ListConfigurationsResponse, error) {
		return &configapi.ListConfigurationsResponse{
			Configs: []string{"delete-a", "agent-1", "delete-b", "delete-c"},
		}, nil
	}

	cli.PutConfigurationFunc = func(_ context.Context, name string, _ *instance.Config) error {
		t.FailNow()
		return nil
	}

	cli.DeleteConfigurationFunc = func(_ context.Context, name string) error {
		t.FailNow()
		return nil
	}

	err := ConfigSync(nil, cli, "./testdata", true)
	require.NoError(t, err)
}

type mockFuncPromClient struct {
	InstancesFunc           func(ctx context.Context) ([]string, error)
	ListConfigsFunc         func(ctx context.Context) (*configapi.ListConfigurationsResponse, error)
	GetConfigurationFunc    func(ctx context.Context, name string) (*instance.Config, error)
	PutConfigurationFunc    func(ctx context.Context, name string, cfg *instance.Config) error
	DeleteConfigurationFunc func(ctx context.Context, name string) error
}

func (m mockFuncPromClient) Instances(ctx context.Context) ([]string, error) {
	if m.InstancesFunc != nil {
		return m.InstancesFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m mockFuncPromClient) ListConfigs(ctx context.Context) (*configapi.ListConfigurationsResponse, error) {
	if m.ListConfigsFunc != nil {
		return m.ListConfigsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m mockFuncPromClient) GetConfiguration(ctx context.Context, name string) (*instance.Config, error) {
	if m.GetConfigurationFunc != nil {
		return m.GetConfigurationFunc(ctx, name)
	}
	return nil, errors.New("not implemented")
}

func (m mockFuncPromClient) PutConfiguration(ctx context.Context, name string, cfg *instance.Config) error {
	if m.PutConfigurationFunc != nil {
		return m.PutConfigurationFunc(ctx, name, cfg)
	}
	return errors.New("not implemented")
}

func (m mockFuncPromClient) DeleteConfiguration(ctx context.Context, name string) error {
	if m.DeleteConfigurationFunc != nil {
		return m.DeleteConfigurationFunc(ctx, name)
	}
	return errors.New("not implemented")
}

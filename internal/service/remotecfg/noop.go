package remotecfg

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	agentv1 "github.com/grafana/agent-remote-config/api/gen/proto/go/agent/v1"
)

type noopClient struct{}

// GetConfig returns the agent's configuration.
func (c noopClient) GetConfig(context.Context, *connect.Request[agentv1.GetConfigRequest]) (*connect.Response[agentv1.GetConfigResponse], error) {
	return nil, errors.New("noop client")
}

// GetAgent returns information about the agent.
func (c noopClient) GetAgent(context.Context, *connect.Request[agentv1.GetAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, errors.New("noop client")
}

// ListAgents returns information about all agents.
func (c noopClient) ListAgents(context.Context, *connect.Request[agentv1.ListAgentsRequest]) (*connect.Response[agentv1.Agents], error) {
	return nil, errors.New("noop client")
}

// CreateAgent registers a new agent.
func (c noopClient) CreateAgent(context.Context, *connect.Request[agentv1.CreateAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, errors.New("noop client")
}

// UpdateAgent updates an existing agent.
func (c noopClient) UpdateAgent(context.Context, *connect.Request[agentv1.UpdateAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, errors.New("noop client")
}

// DeleteAgent deletes an existing agent.
func (c noopClient) DeleteAgent(context.Context, *connect.Request[agentv1.DeleteAgentRequest]) (*connect.Response[agentv1.DeleteAgentResponse], error) {
	return nil, errors.New("noop client")
}

package graph

//go:generate go tool github.com/99designs/gqlgen generate

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

const httpServiceName = "http"

type readyService interface {
	IsReady() bool
}

var componentInfoOptions = component.InfoOptions{
	GetHealth:    true,
	GetArguments: true,
	GetExports:   true,
	GetDebugInfo: true,
}

type Resolver struct {
	Host            service.Host
	CallbackManager livedebugging.CallbackManager
	LogBuffer       *logging.Buffer
	ServiceList     []service.Service
}

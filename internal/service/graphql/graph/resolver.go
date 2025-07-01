//go:generate go run github.com/99designs/gqlgen generate

package graph

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

import (
	"github.com/grafana/alloy/internal/service"
)

type Resolver struct {
	Host service.Host
}

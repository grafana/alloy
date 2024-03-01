package integrations

import (
	"context"
	"net/http"

	"github.com/grafana/agent/internal/util"
)

// FuncIntegration is a function that implements Integration.
type FuncIntegration func(ctx context.Context) error

// RunIntegration implements Integration.
func (fi FuncIntegration) RunIntegration(ctx context.Context) error { return fi(ctx) }

// Handler implements HTTPIntegration
func (fi FuncIntegration) Handler(prefix string) (http.Handler, error) {
	return nil, nil
}

// NoOpIntegration is an Integration that does nothing.
var NoOpIntegration = FuncIntegration(func(ctx context.Context) error {
	<-ctx.Done()
	return nil
})

// CompareConfigs will return true if a and b are equal. If neither a nor b
// implements ComparableConfig, then configs are compared by marshaling to YAML
// and comparing the results.
func CompareConfigs(a, b Config) bool {
	if a, ok := a.(ComparableConfig); ok {
		return a.ConfigEquals(b)
	}
	if b, ok := b.(ComparableConfig); ok {
		return b.ConfigEquals(a)
	}
	return util.CompareYAML(a, b)
}

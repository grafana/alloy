package source

import (
	"context"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runner"
)

var _ SourceFactory = (*basicFactory)(nil)

type basicFactory struct{}

// Sources implements SourceFactory.
func (b *basicFactory) Sources(host Host, args component.Arguments) []Source {
	panic("unimplemented")
}

var _ Source = (*basicSource)(nil)

type basicSource struct {
	hash uint64
	recv loki.LogsReceiver
}

func (b *basicSource) Equals(other runner.Task) bool {
	return b.hash == other.Hash()
}

func (b *basicSource) Hash() uint64 {
	return b.hash
}

func (b *basicSource) Run(ctx context.Context) {
}

func TestComponent(t *testing.T) {

}

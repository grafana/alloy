package source

import (
	"context"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runner"
	"github.com/stretchr/testify/require"
)

var _ SourceFactory = (*basicFactory)(nil)

type basicFactory struct{}

var _ Arguments = (*basicArguments)(nil)

type basicArguments struct {
	forwardTo  []loki.LogsReceiver
	numSources int
}

func (b *basicArguments) Receivers() []loki.LogsReceiver {
	return b.forwardTo
}

func (b *basicFactory) Sources(host Host, args component.Arguments) []Source {
	newArgs := args.(*basicArguments)

	sources := make([]Source, newArgs.numSources)
	for i := range newArgs.numSources {
		sources = append(sources, &basicSource{
			hash: uint64(i),
			host: host,
		})

	}
	return sources
}

var _ Source = (*basicSource)(nil)

type basicSource struct {
	hash uint64
	host Host
}

func (b *basicSource) Equals(other runner.Task) bool {
	return b.hash == other.Hash()
}

func (b *basicSource) Hash() uint64 {
	return b.hash
}

func (b *basicSource) Run(ctx context.Context) {
	for {
		if ok := b.host.Reciever().Send(ctx, loki.Entry{}); !ok {
			break
		}
	}
}

func TestComponent(t *testing.T) {
	_, err := New(component.Options{}, &basicArguments{}, &basicFactory{})
	require.NoError(t, err)
	// FIXME: build tests
}

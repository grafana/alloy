package network

import (
	"context"
	"testing"

	"github.com/vladopajic/go-actor/actor"
)

func BenchmarkMailbox(b *testing.B) {
	// This should be 260 ns roughly or 3m messages a second.
	mbx := actor.NewMailbox[struct{}]()
	mbx.Start()
	defer mbx.Stop()
	go func() {
		for range mbx.ReceiveC() {
		}
	}()
	ctx := context.Background()
	for range b.N {
		mbx.Send(ctx, struct{}{})
	}
}

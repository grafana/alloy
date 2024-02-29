package agentproto

import (
	"context"

	empty "github.com/golang/protobuf/ptypes/empty"
)

// FuncScrapingServiceServer is an implementation of ScrapingServiceServer that
// uses function fields to implement the interface. Useful for tests.
type FuncScrapingServiceServer struct {
	ReshardFunc func(context.Context, *ReshardRequest) (*empty.Empty, error)
}

// Reshard implements ScrapingServiceServer.
func (f *FuncScrapingServiceServer) Reshard(ctx context.Context, req *ReshardRequest) (*empty.Empty, error) {
	if f.ReshardFunc != nil {
		return f.ReshardFunc(ctx, req)
	}
	panic("ReshardFunc is nil")
}

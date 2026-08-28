// Package server implements the HTTP and gRPC server used throughout Grafana
// Agent Static.
//
// It is a grafana/alloy-specific fork of github.com/weaveworks/common/server.
package server

import (
	"context"
	"net"
)

// DialContextFunc is a function matching the signature of
// net.Dialer.DialContext.
type DialContextFunc func(ctx context.Context, network string, addr string) (net.Conn, error)

package graphql

import (
	"context"
	"net/http"
	"path"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph"
	"github.com/vektah/gqlparser/v2/ast"
)

const globalRequestTimeout = 10 * time.Second

type AlloyGraphQLProvider struct {
	srv        *handler.Server
	playground http.Handler
}

func RegisterRoutes(urlPrefix string, r *mux.Router, host service.Host, logger log.Logger, enablePlayground bool) {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	provider := NewAlloyGraphQLProvider(host)

	r.Handle(path.Join(urlPrefix, "/graphql"), provider.srv)

	if enablePlayground {
		level.Info(logger).Log("msg", "GraphQL playground is enabled")
		r.Handle(path.Join(urlPrefix, "/graphql/playground"), provider.playground)
	}
}

func NewAlloyGraphQLProvider(host service.Host) *AlloyGraphQLProvider {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{
		Host: host,
	}}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	// Cache parsed queries as their AST
	srv.SetQueryCache(lru.New[*ast.QueryDocument](100))

	srv.Use(extension.Introspection{})
	// This is only useful for large queries at high volume. Should that become needed, the following
	// could be turned on.
	// srv.Use(extension.AutomaticPersistedQuery{
	// 	Cache: lru.New[string](100),
	// })

	// Add global timeout for all GraphQL operations
	srv.AroundRootFields(func(ctx context.Context, next graphql.RootResolver) graphql.Marshaler {
		timeoutCtx, cancel := context.WithTimeout(ctx, globalRequestTimeout)
		defer cancel()
		return next(timeoutCtx)
	})

	return &AlloyGraphQLProvider{
		srv:        srv,
		playground: playground.Handler("GraphQL playground", "/graphql"),
	}
}

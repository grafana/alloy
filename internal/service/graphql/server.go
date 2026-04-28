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

type RegisterRoutesParams struct {
	Router           *mux.Router
	Logger           log.Logger
	URLPrefix        string
	Host             service.Host
	EnablePlayground bool
}

func RegisterRoutes(params RegisterRoutesParams) {
	if params.Logger == nil {
		params.Logger = log.NewNopLogger()
	}

	provider := newAlloyGraphQLProvider(params.URLPrefix, params.Host, params.EnablePlayground)

	params.Router.Handle(path.Join(params.URLPrefix, "/graphql"), provider.srv)

	if params.EnablePlayground {
		level.Info(params.Logger).Log("msg", "GraphQL playground is enabled")
		params.Router.Handle(path.Join(params.URLPrefix, "/graphql/playground"), provider.playground)
	}
}

func newAlloyGraphQLProvider(urlPrefix string, host service.Host, enablePlayground bool) *AlloyGraphQLProvider {
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

	provider := &AlloyGraphQLProvider{srv: srv}
	if enablePlayground {
		provider.playground = playground.Handler("GraphQL playground", path.Join(urlPrefix, "/graphql"))
	}
	return provider
}

package graphql

import (
	"context"
	"net/http"
	"os"
	"path"
	"strings"
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

func RegisterRoutes(urlPrefix string, r *mux.Router, host service.Host, logger log.Logger) {
	if logger == nil {
		logger = log.NewNopLogger()
	}
	provider := NewAlloyGraphQLProvider(host)

	r.Handle(path.Join(urlPrefix, "/graphql"), provider.srv)

	// Only register the playground if explicitly enabled
	v := strings.ToLower(strings.TrimSpace(os.Getenv("ALLOY_ENABLE_GRAPHQL_PLAYGROUND")))

	var playgroundEnabled = map[string]struct{}{
		"1":    {},
		"true": {},
		"yes":  {},
	}
	if _, enabled := playgroundEnabled[v]; enabled {
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

	srv.SetQueryCache(lru.New[*ast.QueryDocument](100))

	srv.Use(extension.Introspection{})
	// It's unlikely we will need caching of queries, but given sufficient query volume, this could be
	// turned on to reduce CPU at the expense of memory.
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

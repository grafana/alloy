package graphql

import (
	"net/http"
	"path"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/graphql/graph"
	"github.com/vektah/gqlparser/v2/ast"
)

const defaultPort = "8080"

type AlloyGraphQLProvider struct {
	srv        *handler.Server
	playground http.Handler
}

func RegisterRoutes(urlPrefix string, r *mux.Router, host service.Host) {
	provider := NewAlloyGraphQLProvider(host)

	r.Handle(path.Join(urlPrefix, "/graphql"), provider.srv)
	// TODO: Only register the playground in development mode (or similar)
	r.Handle(path.Join(urlPrefix, "/graphql/playground"), provider.playground)
}

func NewAlloyGraphQLProvider(host service.Host) *AlloyGraphQLProvider {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{
		Host: host,
	}}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	return &AlloyGraphQLProvider{
		srv:        srv,
		playground: playground.Handler("GraphQL playground", "/graphql"),
	}
}

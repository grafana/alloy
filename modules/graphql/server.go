package graphql

import (
	"log"
	"net/http"
	"os"
	"path"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/modules/graphql/graph"
	"github.com/vektah/gqlparser/v2/ast"
)

const defaultPort = "8080"

type AlloyGraphQLProvider struct {
	srv        *handler.Server
	playground http.Handler
}

func RegisterRoutes(urlPrefix string, r *mux.Router) {
	provider := NewAlloyGraphQLProvider()

	r.Handle(path.Join(urlPrefix, "/graphql"), provider.srv)
	// TODO: Only register the playground in development mode (or similar)
	r.Handle(path.Join(urlPrefix, "/graphql/playground"), provider.playground)
}

func NewAlloyGraphQLProvider() *AlloyGraphQLProvider {
	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

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

// Runs just the graphql server without the rest of Alloy.
func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{}}))

	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})

	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", srv)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

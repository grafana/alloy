package harness

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "pipelinetest.mocks.http",
		Stability: featuregate.StabilityExperimental,
		Args:      MockHTTPArguments{},
		Exports:   MockHTTPExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewMockHTTP(opts, args.(MockHTTPArguments))
		},
	})
}

// MockHTTPArguments configures a stub HTTP server with canned responses.
type MockHTTPArguments struct {
	Routes []MockHTTPRoute `alloy:"route,block,optional"`
}

// MockHTTPRoute is one canned response. A request matches when its path equals
// Path and, if Method is set, its method equals Method. An unset Status means
// 200.
type MockHTTPRoute struct {
	Path    string            `alloy:"path,attr"`
	Method  string            `alloy:"method,attr,optional"`
	Status  int               `alloy:"status,attr,optional"`
	Headers map[string]string `alloy:"headers,attr,optional"`
	Body    string            `alloy:"body,attr,optional"`
}

// Validate implements syntax.Validator.
func (a *MockHTTPArguments) Validate() error {
	seen := make(map[string]struct{}, len(a.Routes))
	for _, route := range a.Routes {
		if !strings.HasPrefix(route.Path, "/") {
			return fmt.Errorf("route path %q must start with /", route.Path)
		}
		if route.Status != 0 && (route.Status < 100 || route.Status > 599) {
			return fmt.Errorf("route %s: status %d is not a valid HTTP status code", route.Path, route.Status)
		}

		key := strings.ToUpper(route.Method) + " " + route.Path
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate route for method %q and path %q", route.Method, route.Path)
		}
		seen[key] = struct{}{}
	}
	return nil
}

type MockHTTPExports struct {
	URL string `alloy:"url,attr"`
}

// MockHTTP serves canned responses so a pipeline test can point components at
// an upstream without running a real one. Requests that match no route get a
// 404.
type MockHTTP struct {
	server *httptest.Server

	mux    sync.RWMutex
	routes []MockHTTPRoute
}

func NewMockHTTP(opts component.Options, args MockHTTPArguments) (*MockHTTP, error) {
	m := &MockHTTP{routes: args.Routes}
	m.server = httptest.NewServer(http.HandlerFunc(m.serve))

	opts.OnStateChange(MockHTTPExports{URL: m.server.URL})
	return m, nil
}

var _ component.Component = (*MockHTTP)(nil)

func (m *MockHTTP) Run(ctx context.Context) error {
	defer m.server.Close()
	<-ctx.Done()
	return nil
}

func (m *MockHTTP) Update(args component.Arguments) error {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.routes = args.(MockHTTPArguments).Routes
	return nil
}

func (m *MockHTTP) serve(w http.ResponseWriter, r *http.Request) {
	route, ok := m.match(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	for k, v := range route.Headers {
		w.Header().Set(k, v)
	}

	status := route.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write([]byte(route.Body))
}

// match returns the route for r. A route with a method set wins over one that
// accepts any method on the same path.
func (m *MockHTTP) match(r *http.Request) (MockHTTPRoute, bool) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	var (
		anyMethod MockHTTPRoute
		found     bool
	)
	for _, route := range m.routes {
		if route.Path != r.URL.Path {
			continue
		}
		if route.Method == "" {
			anyMethod, found = route, true
			continue
		}
		if strings.EqualFold(route.Method, r.Method) {
			return route, true
		}
	}
	return anyMethod, found
}

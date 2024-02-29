package importsource

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/grafana/agent/internal/component"
	common_config "github.com/grafana/agent/internal/component/common/config"
	remote_http "github.com/grafana/agent/internal/component/remote/http"
	"github.com/grafana/river/vm"
)

// ImportHTTP imports a module from a HTTP server via the remote.http component.
type ImportHTTP struct {
	managedRemoteHTTP *remote_http.Component
	arguments         component.Arguments
	managedOpts       component.Options
	eval              *vm.Evaluator
}

var _ ImportSource = (*ImportHTTP)(nil)

func NewImportHTTP(managedOpts component.Options, eval *vm.Evaluator, onContentChange func(string)) *ImportHTTP {
	opts := managedOpts
	opts.OnStateChange = func(e component.Exports) {
		onContentChange(e.(remote_http.Exports).Content.Value)
	}
	return &ImportHTTP{
		managedOpts: opts,
		eval:        eval,
	}
}

// HTTPArguments holds values which are used to configure the remote.http component.
type HTTPArguments struct {
	URL           string        `river:"url,attr"`
	PollFrequency time.Duration `river:"poll_frequency,attr,optional"`
	PollTimeout   time.Duration `river:"poll_timeout,attr,optional"`

	Method  string            `river:"method,attr,optional"`
	Headers map[string]string `river:"headers,attr,optional"`
	Body    string            `river:"body,attr,optional"`

	Client common_config.HTTPClientConfig `river:"client,block,optional"`
}

// DefaultHTTPArguments holds default settings for HTTPArguments.
var DefaultHTTPArguments = HTTPArguments{
	PollFrequency: 1 * time.Minute,
	PollTimeout:   10 * time.Second,
	Client:        common_config.DefaultHTTPClientConfig,
	Method:        http.MethodGet,
}

// SetToDefault implements river.Defaulter.
func (args *HTTPArguments) SetToDefault() {
	*args = DefaultHTTPArguments
}

func (im *ImportHTTP) Evaluate(scope *vm.Scope) error {
	var arguments HTTPArguments
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding River: %w", err)
	}
	if im.managedRemoteHTTP == nil {
		var err error
		im.managedRemoteHTTP, err = remote_http.New(im.managedOpts, remote_http.Arguments{
			URL:           arguments.URL,
			PollFrequency: arguments.PollFrequency,
			PollTimeout:   arguments.PollTimeout,
			Method:        arguments.Method,
			Headers:       arguments.Headers,
			Body:          arguments.Body,
			Client:        arguments.Client,
		})
		if err != nil {
			return fmt.Errorf("creating http component: %w", err)
		}
		im.arguments = arguments
	}

	if reflect.DeepEqual(im.arguments, arguments) {
		return nil
	}

	// Update the existing managed component
	if err := im.managedRemoteHTTP.Update(arguments); err != nil {
		return fmt.Errorf("updating component: %w", err)
	}
	im.arguments = arguments
	return nil
}

func (im *ImportHTTP) Run(ctx context.Context) error {
	return im.managedRemoteHTTP.Run(ctx)
}

func (im *ImportHTTP) CurrentHealth() component.Health {
	return im.managedRemoteHTTP.CurrentHealth()
}

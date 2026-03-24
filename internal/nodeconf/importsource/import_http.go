package importsource

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/grafana/alloy/internal/component"
	common_config "github.com/grafana/alloy/internal/component/common/config"
	remote_http "github.com/grafana/alloy/internal/component/remote/http"
	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/syntax/vm"
)

// ImportHTTP imports a module from a HTTP server via the remote.http component.
type ImportHTTP struct {
	managedRemoteHTTP *remote_http.Component
	arguments         HTTPArguments
	managedOpts       component.Options
	eval              *vm.Evaluator
}

var _ ImportSource = (*ImportHTTP)(nil)

func NewImportHTTP(managedOpts component.Options, eval *vm.Evaluator, onContentChange func(map[string]string)) *ImportHTTP {
	opts := managedOpts
	opts.OnStateChange = func(e component.Exports) {
		onContentChange(map[string]string{opts.ID: e.(remote_http.Exports).Content.Value})
	}
	return &ImportHTTP{
		managedOpts: opts,
		eval:        eval,
	}
}

// HTTPArguments holds values which are used to configure the remote.http component.
type HTTPArguments struct {
	URL           string        `alloy:"url,attr"`
	PollFrequency time.Duration `alloy:"poll_frequency,attr,optional"`
	PollTimeout   time.Duration `alloy:"poll_timeout,attr,optional"`

	Method  string            `alloy:"method,attr,optional"`
	Headers map[string]string `alloy:"headers,attr,optional"`
	Body    string            `alloy:"body,attr,optional"`

	Client common_config.HTTPClientConfig `alloy:"client,block,optional"`
}

// DefaultHTTPArguments holds default settings for HTTPArguments.
var DefaultHTTPArguments = HTTPArguments{
	PollFrequency: 1 * time.Minute,
	PollTimeout:   10 * time.Second,
	Client:        common_config.DefaultHTTPClientConfig,
	Method:        http.MethodGet,
}

// SetToDefault implements syntax.Defaulter.
func (args *HTTPArguments) SetToDefault() {
	*args = DefaultHTTPArguments
}

func (im *ImportHTTP) Evaluate(scope *vm.Scope) error {
	var arguments HTTPArguments
	if err := im.eval.Evaluate(scope, &arguments); err != nil {
		return fmt.Errorf("decoding configuration: %w", err)
	}
	remoteHttpArguments := remote_http.Arguments{
		URL:           arguments.URL,
		PollFrequency: arguments.PollFrequency,
		PollTimeout:   arguments.PollTimeout,
		Method:        arguments.Method,
		Headers:       arguments.Headers,
		Body:          arguments.Body,
		Client:        arguments.Client,
	}
	if im.managedRemoteHTTP == nil {
		var err error
		im.managedRemoteHTTP, err = remote_http.New(im.managedOpts, remoteHttpArguments)
		if err != nil {
			return fmt.Errorf("creating http component: %w", err)
		}
		im.arguments = arguments
	}

	if equality.DeepEqual(im.arguments, arguments) {
		return nil
	}

	// Update the existing managed component
	if err := im.managedRemoteHTTP.Update(remoteHttpArguments); err != nil {
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

// Update the evaluator.
func (im *ImportHTTP) SetEval(eval *vm.Evaluator) {
	im.eval = eval
}

func (im *ImportHTTP) ModulePath() string {
	dir, _ := path.Split(im.arguments.URL)
	return dir
}

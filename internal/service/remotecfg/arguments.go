package remotecfg

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/alloyseed"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/syntax"
)

// Arguments holds runtime settings for the remotecfg service.
type Arguments struct {
	URL              string                   `alloy:"url,attr,optional"`
	ID               string                   `alloy:"id,attr,optional"`
	Name             string                   `alloy:"name,attr,optional"`
	Attributes       map[string]string        `alloy:"attributes,attr,optional"`
	PollFrequency    time.Duration            `alloy:"poll_frequency,attr,optional"`
	HTTPClientConfig *config.HTTPClientConfig `alloy:",squash"`
}

// Make sure Arguments implements the syntax.Defaulter interface
var _ syntax.Defaulter = (*Arguments)(nil)

// Make sure Arguments implements the syntax.Validator interface
var _ syntax.Validator = (*Arguments)(nil)

// getDefaultArguments populates the default values for the Arguments struct.
func getDefaultArguments() Arguments {
	return Arguments{
		ID:               alloyseed.Get().UID,
		Attributes:       make(map[string]string),
		PollFrequency:    1 * time.Minute,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = getDefaultArguments()
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.PollFrequency < 10*time.Second {
		return fmt.Errorf("poll_frequency must be at least \"10s\", got %q", a.PollFrequency)
	}

	for k := range a.Attributes {
		if strings.HasPrefix(k, reservedAttributeNamespace+namespaceDelimiter) {
			return fmt.Errorf("%q is a reserved namespace for remotecfg attribute keys", reservedAttributeNamespace)
		}
	}

	// We must explicitly Validate because HTTPClientConfig is squashed and it
	// won't run otherwise
	if a.HTTPClientConfig != nil {
		return a.HTTPClientConfig.Validate()
	}

	return nil
}

// Hash marshals the Arguments and returns a hash representation.
func (a *Arguments) Hash() (string, error) {
	b, err := syntax.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("failed to marshal arguments: %w", err)
	}
	return getHash(b), nil
}

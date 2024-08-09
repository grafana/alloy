package s3

import (
	"fmt"
	"time"

	aws_common_config "github.com/grafana/alloy/internal/component/common/config/aws"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// Arguments implements the input for the S3 component.
type Arguments struct {
	Path string `alloy:"path,attr"`
	// PollFrequency determines the frequency to check for changes
	// defaults to 10m.
	PollFrequency time.Duration `alloy:"poll_frequency,attr,optional"`
	// IsSecret determines if the content should be displayed to the user.
	IsSecret bool `alloy:"is_secret,attr,optional"`
	// Options allows the overriding of default AWS settings.
	Options Client `alloy:"client,block,optional"`
}

type Client struct {
	aws_common_config.Client
	UsePathStyle bool `alloy:"use_path_style,attr,optional"`
}

const minimumPollFrequency = 30 * time.Second

// DefaultArguments sets the poll frequency
var DefaultArguments = Arguments{
	PollFrequency: 10 * time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.PollFrequency <= minimumPollFrequency {
		return fmt.Errorf("poll_frequency must be greater than 30s")
	}
	return nil
}

// Exports implements the file content
type Exports struct {
	Content alloytypes.OptionalSecret `alloy:"content,attr"`
}

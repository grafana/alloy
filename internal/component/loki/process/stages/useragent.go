package stages

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
	"github.com/ua-parser/uap-go/uaparser"
)

// Config Errors.
var (
	ErrEmptyUserAgentStageSource = errors.New("empty source")
)

// UserAgentConfig configures a processing stage that uses uap-core to
// parse user-agent strings and extract browser, OS, and device information.
type UserAgentConfig struct {
	Source    *string `alloy:"source,attr,optional"`
	RegexFile string  `alloy:"regex_file,attr,optional"`
}

// validateUserAgentConfig validates the config
func validateUserAgentConfig(c UserAgentConfig) error {
	if c.Source != nil && *c.Source == "" {
		return ErrEmptyUserAgentStageSource
	}
	return nil
}

// userAgentStage parses user-agent strings and extracts browser/OS/device info
type userAgentStage struct {
	config *UserAgentConfig
	parser *uaparser.Parser
	logger log.Logger
}

// newUserAgentStage creates a newUserAgentStage
func newUserAgentStage(logger log.Logger, config UserAgentConfig) (Stage, error) {
	if err := validateUserAgentConfig(config); err != nil {
		return nil, err
	}

	var parser *uaparser.Parser
	if config.RegexFile != "" {
		var err error
		parser, err = uaparser.New(config.RegexFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load regex file %s: %w", config.RegexFile, err)
		}
	} else {
		parser = uaparser.NewFromSaved()
	}

	return toStage(&userAgentStage{
		config: &config,
		parser: parser,
		logger: log.With(logger, "component", "stage", "type", "useragent"),
	}), nil
}

// Process implements Stage
func (u *userAgentStage) Process(labels model.LabelSet, extracted map[string]interface{}, t *time.Time, entry *string) {
	// If a source key is provided, the user_agent stage should process it
	// from the extracted map, otherwise should fall back to the entry
	input := entry

	if u.config.Source != nil {
		if _, ok := extracted[*u.config.Source]; !ok {
			if Debug {
				level.Debug(u.logger).Log("msg", "source does not exist in the set of extracted values", "source", *u.config.Source)
			}
			return
		}

		value, err := getString(extracted[*u.config.Source])
		if err != nil {
			if Debug {
				level.Debug(u.logger).Log("msg", "failed to convert source value to string", "source", *u.config.Source, "err", err, "type", reflect.TypeOf(extracted[*u.config.Source]))
			}
			return
		}

		input = &value
	}

	if input == nil {
		if Debug {
			level.Debug(u.logger).Log("msg", "cannot parse a nil entry")
		}
		return
	}

	// Parse the user-agent string
	client := u.parser.Parse(*input)

	// Extract browser information
	if client.UserAgent.Family != "" {
		extracted["useragent_browser"] = client.UserAgent.Family
	}

	if client.UserAgent.Major != "" {
		extracted["useragent_browser_version"] = fmt.Sprintf("%s.%s.%s", client.UserAgent.Major, client.UserAgent.Minor, client.UserAgent.Patch)
	}

	// Extract OS information
	if client.Os.Family != "" {
		extracted["useragent_os"] = client.Os.Family
	}

	if client.Os.Major != "" {
		extracted["useragent_os_version"] = fmt.Sprintf("%s.%s.%s", client.Os.Major, client.Os.Minor, client.Os.Patch)
	}

	// Extract device information
	if client.Device.Family != "" && client.Device.Family != "Other" {
		extracted["useragent_device"] = client.Device.Family
	}

	if client.Device.Brand != "" {
		extracted["useragent_device_brand"] = client.Device.Brand
	}

	if client.Device.Model != "" {
		extracted["useragent_device_model"] = client.Device.Model
	}

	if Debug {
		level.Debug(u.logger).Log("msg", "extracted user-agent data debug", "extracted data", fmt.Sprintf("%v", extracted))
	}
}

// Name implements Stage
func (u *userAgentStage) Name() string {
	return StageTypeUserAgent
}

package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	gaugeSet = "set"
	gaugeInc = "inc"
	gaugeDec = "dec"
	gaugeAdd = "add"
	gaugeSub = "sub"

	errGaugeActionRequired = "gauge action must be defined as `set`, `inc`, `dec`, `add`, or `sub`"
	errGaugeInvalidAction  = "action %s is not valid, action must be `set`, `inc`, `dec`, `add`, or `sub`"
)

type GaugeConfig struct {
	Value  *string `mapstructure:"value"`
	Action string  `mapstructure:"action"`
}

func ParseGaugeConfig(config any) (*GaugeConfig, error) {
	cfg := &GaugeConfig{}

	if err := mapstructure.Decode(config, cfg); err != nil {
		return nil, err
	}

	if err := validateGaugeConfig(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateGaugeConfig(config *GaugeConfig) error {
	if config.Action == "" {
		return errors.New(errGaugeActionRequired)
	}
	config.Action = strings.ToLower(config.Action)
	if config.Action != gaugeSet &&
		config.Action != gaugeInc &&
		config.Action != gaugeDec &&
		config.Action != gaugeAdd &&
		config.Action != gaugeSub {

		return fmt.Errorf(errGaugeInvalidAction, config.Action)
	}
	return nil
}

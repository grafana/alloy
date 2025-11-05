package metric

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"
)

const (
	GaugeSet = "set"
	GaugeInc = "inc"
	GaugeDec = "dec"
	GaugeAdd = "add"
	GaugeSub = "sub"

	ErrGaugeActionRequired = "gauge action must be defined as `set`, `inc`, `dec`, `add`, or `sub`"
	ErrGaugeInvalidAction  = "action %s is not valid, action must be `set`, `inc`, `dec`, `add`, or `sub`"
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
		return errors.New(ErrGaugeActionRequired)
	}
	config.Action = strings.ToLower(config.Action)
	if config.Action != GaugeSet &&
		config.Action != GaugeInc &&
		config.Action != GaugeDec &&
		config.Action != GaugeAdd &&
		config.Action != GaugeSub {

		return fmt.Errorf(ErrGaugeInvalidAction, config.Action)
	}
	return nil
}

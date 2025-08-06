// NOTE: this file is copied from Prometheus codebase and adapted to work correctly with Alloy types.
// For backwards compatibility purposes, the behaviour implemented here should not be changed.

// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package relabel

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/regexp"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

// Action is the relabelling action to be performed.
type Action string

// All possible Action values.
const (
	Replace   Action = "replace"
	Keep      Action = "keep"
	Drop      Action = "drop"
	HashMod   Action = "hashmod"
	LabelMap  Action = "labelmap"
	LabelDrop Action = "labeldrop"
	LabelKeep Action = "labelkeep"
	Lowercase Action = "lowercase"
	Uppercase Action = "uppercase"
	KeepEqual Action = "keepequal"
	DropEqual Action = "dropequal"
)

var actions = map[Action]struct{}{
	Replace:   {},
	Keep:      {},
	Drop:      {},
	HashMod:   {},
	LabelMap:  {},
	LabelDrop: {},
	LabelKeep: {},
	Lowercase: {},
	Uppercase: {},
	KeepEqual: {},
	DropEqual: {},
}

// String returns the string representation of the Action type.
func (a Action) String() string {
	if _, exists := actions[a]; exists {
		return string(a)
	}
	return "Action:" + string(a)
}

// MarshalText implements encoding.TextMarshaler for Action.
func (a Action) MarshalText() (text []byte, err error) {
	return []byte(a.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for Action.
func (a *Action) UnmarshalText(text []byte) error {
	if _, exists := actions[Action(text)]; exists {
		*a = Action(text)
		return nil
	}
	return fmt.Errorf("unrecognized action type %q", string(text))
}

// Regexp encapsulates the Regexp type from Grafana's fork of the Go stdlib regexp package.
type Regexp struct {
	*regexp.Regexp
}

func newRegexp(s string) (Regexp, error) {
	regex, err := regexp.Compile("^(?s:" + s + ")$")
	return Regexp{regex}, err
}

func mustNewRegexp(s string) Regexp {
	re, err := newRegexp(s)
	if err != nil {
		panic(err)
	}
	return re
}

// MarshalText implements encoding.TextMarshaler for Regexp.
func (re Regexp) MarshalText() (text []byte, err error) {
	if re.String() != "" {
		return []byte(re.String()), nil
	}
	return nil, nil
}

// UnmarshalText implements encoding.TextUnmarshaler for Regexp.
func (re *Regexp) UnmarshalText(text []byte) error {
	r, err := newRegexp(string(text))
	if err != nil {
		return err
	}
	*re = r
	return nil
}

// String returns the original string used to compile the regular expression.
func (re Regexp) String() string {
	if re.Regexp == nil {
		return ""
	}

	str := re.Regexp.String()
	// Trim the anchor `^(?s:` prefix and `)$` suffix.
	return str[5 : len(str)-2]
}

// Config describes a relabelling step to be applied on a target.
type Config struct {
	SourceLabels []string `alloy:"source_labels,attr,optional"`
	Separator    string   `alloy:"separator,attr,optional"`
	Regex        Regexp   `alloy:"regex,attr,optional"`
	Modulus      uint64   `alloy:"modulus,attr,optional"`
	TargetLabel  string   `alloy:"target_label,attr,optional"`
	Replacement  string   `alloy:"replacement,attr,optional"`
	Action       Action   `alloy:"action,attr,optional"`
}

// DefaultRelabelConfig sets the default values of fields when decoding a RelabelConfig block.
var DefaultRelabelConfig = Config{
	Action:      Replace,
	Separator:   ";",
	Regex:       mustNewRegexp("(.*)"),
	Replacement: "$1",
}

// SetToDefault implements syntax.Defaulter.
func (c *Config) SetToDefault() {
	*c = Config{
		Action:      Replace,
		Separator:   ";",
		Regex:       mustNewRegexp("(.*)"),
		Replacement: "$1",
	}
}

var relabelTarget = regexp.MustCompile(`^(?:(?:[a-zA-Z_]|\$(?:\{\w+\}|\w+))+\w*)+$`)

// Validate implements syntax.Validator.
func (rc *Config) Validate() error {
	if rc.Action == "" {
		return fmt.Errorf("relabel action cannot be empty")
	}
	if rc.Modulus == 0 && rc.Action == HashMod {
		return fmt.Errorf("relabel configuration for hashmod requires non-zero modulus")
	}
	if (rc.Action == Replace || rc.Action == HashMod || rc.Action == Lowercase || rc.Action == Uppercase || rc.Action == KeepEqual || rc.Action == DropEqual) && rc.TargetLabel == "" {
		return fmt.Errorf("relabel configuration for %s action requires 'target_label' value", rc.Action)
	}
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if rc.Action == Replace && !strings.Contains(rc.TargetLabel, "$") && !model.LabelName(rc.TargetLabel).IsValid() {
		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
	}
	if rc.Action == Replace && strings.Contains(rc.TargetLabel, "$") && !relabelTarget.MatchString(rc.TargetLabel) {
		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
	}
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if (rc.Action == Lowercase || rc.Action == Uppercase || rc.Action == KeepEqual || rc.Action == DropEqual) && !model.LabelName(rc.TargetLabel).IsValid() {
		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
	}
	if (rc.Action == Lowercase || rc.Action == Uppercase || rc.Action == KeepEqual || rc.Action == DropEqual) && rc.Replacement != DefaultRelabelConfig.Replacement {
		return fmt.Errorf("'replacement' can not be set for %s action", rc.Action)
	}
	if rc.Action == LabelMap && !relabelTarget.MatchString(rc.Replacement) {
		return fmt.Errorf("%q is invalid 'replacement' for %s action", rc.Replacement, rc.Action)
	}
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if rc.Action == HashMod && !model.LabelName(rc.TargetLabel).IsValid() {
		return fmt.Errorf("%q is invalid 'target_label' for %s action", rc.TargetLabel, rc.Action)
	}

	if rc.Action == LabelDrop || rc.Action == LabelKeep {
		if rc.SourceLabels != nil ||
			rc.TargetLabel != DefaultRelabelConfig.TargetLabel ||
			rc.Modulus != DefaultRelabelConfig.Modulus ||
			rc.Separator != DefaultRelabelConfig.Separator ||
			rc.Replacement != DefaultRelabelConfig.Replacement {

			return fmt.Errorf("%s action requires only 'regex', and no other fields", rc.Action)
		}
	}

	if rc.Action == KeepEqual || rc.Action == DropEqual {
		if !reflect.DeepEqual(*rc.Regex.Regexp, *DefaultRelabelConfig.Regex.Regexp) ||
			rc.Modulus != DefaultRelabelConfig.Modulus ||
			rc.Separator != DefaultRelabelConfig.Separator ||
			rc.Replacement != DefaultRelabelConfig.Replacement {

			return fmt.Errorf("%s action requires only 'source_labels' and 'target_label', and no other fields", rc.Action)
		}
	}

	return nil
}

// ProcessBuilder should be called with lb LabelBuilder containing the initial set of labels,
// which are then modified following the configured rules using builder's methods such as Set and Del.
func ProcessBuilder(lb LabelBuilder, cfgs ...*Config) (keep bool) {
	for _, cfg := range cfgs {
		keep = doRelabel(cfg, lb)
		if !keep {
			return false
		}
	}
	return true
}

func doRelabel(cfg *Config, lb LabelBuilder) (keep bool) {
	var va [16]string
	values := va[:0]
	if len(cfg.SourceLabels) > cap(values) {
		values = make([]string, 0, len(cfg.SourceLabels))
	}
	for _, ln := range cfg.SourceLabels {
		values = append(values, lb.Get(ln))
	}
	val := strings.Join(values, cfg.Separator)

	switch cfg.Action {
	case Drop:
		if cfg.Regex.MatchString(val) {
			return false
		}
	case Keep:
		if !cfg.Regex.MatchString(val) {
			return false
		}
	case DropEqual:
		if lb.Get(cfg.TargetLabel) == val {
			return false
		}
	case KeepEqual:
		if lb.Get(cfg.TargetLabel) != val {
			return false
		}
	case Replace:
		indexes := cfg.Regex.FindStringSubmatchIndex(val)
		// If there is no match no replacement must take place.
		if indexes == nil {
			break
		}
		target := model.LabelName(cfg.Regex.ExpandString([]byte{}, cfg.TargetLabel, val, indexes))
		// TODO: add support for different validation schemes.
		//nolint:staticcheck
		if !target.IsValid() {
			break
		}
		res := cfg.Regex.ExpandString([]byte{}, cfg.Replacement, val, indexes)
		if len(res) == 0 {
			lb.Del(string(target))
			break
		}
		lb.Set(string(target), string(res))
	case Lowercase:
		lb.Set(cfg.TargetLabel, strings.ToLower(val))
	case Uppercase:
		lb.Set(cfg.TargetLabel, strings.ToUpper(val))
	case HashMod:
		hash := md5.Sum([]byte(val))
		// Use only the last 8 bytes of the hash to give the same result as earlier versions of this code.
		mod := binary.BigEndian.Uint64(hash[8:]) % cfg.Modulus
		lb.Set(cfg.TargetLabel, fmt.Sprintf("%d", mod))
	case LabelMap:
		lb.Range(func(name, value string) {
			if cfg.Regex.MatchString(name) {
				res := cfg.Regex.ReplaceAllString(name, cfg.Replacement)
				lb.Set(res, value)
			}
		})
	case LabelDrop:
		lb.Range(func(name, value string) {
			if cfg.Regex.MatchString(name) {
				lb.Del(name)
			}
		})
	case LabelKeep:
		lb.Range(func(name, value string) {
			if !cfg.Regex.MatchString(name) {
				lb.Del(name)
			}
		})
	default:
		panic(fmt.Errorf("relabel: unknown relabel action type %q", cfg.Action))
	}

	return true
}

// ComponentToPromRelabelConfigs bridges the Component-based configuration of
// relabeling steps to the Prometheus implementation.
func ComponentToPromRelabelConfigs(rcs []*Config) []*relabel.Config {
	res := make([]*relabel.Config, len(rcs))
	for i, rc := range rcs {
		sourceLabels := make([]model.LabelName, len(rc.SourceLabels))
		for i, sl := range rc.SourceLabels {
			sourceLabels[i] = model.LabelName(sl)
		}

		res[i] = &relabel.Config{
			SourceLabels: sourceLabels,
			Separator:    rc.Separator,
			Modulus:      rc.Modulus,
			TargetLabel:  rc.TargetLabel,
			Replacement:  rc.Replacement,
			Action:       relabel.Action(rc.Action),
			Regex:        relabel.Regexp{Regexp: rc.Regex.Regexp},
		}
	}

	return res
}

// Rules returns the relabel configs in use for a relabeling component.
type Rules []*Config

// AlloyCapsule marks the alias defined above as a "capsule type" so that it
// cannot be invoked by Alloy code.
func (r Rules) AlloyCapsule() {}

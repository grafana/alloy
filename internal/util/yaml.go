package util

import (
	"errors"
	"regexp"
	"sort"

	"gopkg.in/yaml.v2"
)

// RawYAML is similar to json.RawMessage and allows for deferred YAML decoding.
type RawYAML []byte

// UnmarshalYAML implements yaml.Unmarshaler.
func (r *RawYAML) UnmarshalYAML(unmarshal func(any) error) error {
	var ms yaml.MapSlice
	if err := unmarshal(&ms); err != nil {
		return err
	}
	bb, err := yaml.Marshal(ms)
	if err != nil {
		return err
	}
	*r = bb
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (r RawYAML) MarshalYAML() (any, error) {
	return r.Map()
}

// Map converts the raw YAML into a yaml.MapSlice.
func (r RawYAML) Map() (yaml.MapSlice, error) {
	var ms yaml.MapSlice
	if err := yaml.Unmarshal(r, &ms); err != nil {
		return nil, err
	}
	return ms, nil
}

// MarshalYAMLMerged marshals all values from vv into a single object.
func MarshalYAMLMerged(vv ...any) ([]byte, error) {
	var full yaml.MapSlice
	for _, v := range vv {
		bb, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}
		ms, err := RawYAML(bb).Map()
		if err != nil {
			return nil, err
		}
		full = append(full, ms...)
	}
	return yaml.Marshal(full)
}

// UnmarshalYAMLMerged performs a strict unmarshal of bb into all values from
// vv.
func UnmarshalYAMLMerged(bb []byte, vv ...any) error {
	var typeErrors []yaml.TypeError

	for _, v := range vv {
		// Perform a strict unmarshal. This is likely to fail with type errors for
		// missing fields that may have been consumed by another object in vv.
		var te *yaml.TypeError
		if err := yaml.UnmarshalStrict(bb, v); errors.As(err, &te) {
			typeErrors = append(typeErrors, *te)
		} else if err != nil {
			return err
		}

		// It's common for custom yaml.Unmarshaler implementations to use
		// UnmarshalYAML to apply default values both before and after calling the
		// unmarshal method passed to them.
		//
		// We *must* do a second non-strict unmarshal *after* the strict unmarshal
		// to ensure that every v was able to complete its unmarshal to completion,
		// ignoring type errors from unrecognized fields.
		if err := yaml.Unmarshal(bb, v); err != nil {
			return err
		}
	}

	var (
		addedErrors    = map[string]struct{}{}
		notFoundErrors = map[string]int{}
	)

	// Do an initial pass over our errors, separating errors for not found fields.
	// Other errors are "real" and should be returned.
	for _, te := range typeErrors {
		for _, msg := range te.Errors {
			notFound := notFoundErrRegex.FindStringSubmatch(msg)
			if notFound != nil {
				// Track the invalid field error. Use the first capture group which
				// excludes the type, which will be unique per v.
				notFoundErrors[notFound[1]]++
				continue
			}
			addedErrors[msg] = struct{}{}
		}
	}

	// Iterate over our errors for not found fields. The field truly isn't defined
	// if it was reported len(vv) times (i.e., no v consumed it).
	for msg, count := range notFoundErrors {
		if count == len(vv) {
			addedErrors[msg] = struct{}{}
		}
	}

	if len(addedErrors) > 0 {
		realErrors := make([]string, 0, len(addedErrors))
		for msg := range addedErrors {
			realErrors = append(realErrors, msg)
		}
		sort.Strings(realErrors)
		return &yaml.TypeError{Errors: realErrors}
	}
	return nil
}

var notFoundErrRegex = regexp.MustCompile(`^(line \d+: field .* not found) in type .*$`)

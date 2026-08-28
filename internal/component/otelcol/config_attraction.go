package otelcol

import (
	"errors"
	"fmt"
	"strings"
)

const (
	delete = "delete"
	hash   = "hash"
)

type AttrActionKeyValueSlice []AttrActionKeyValue

func (actions AttrActionKeyValueSlice) Convert() []any {
	res := make([]any, 0, len(actions))

	if len(actions) == 0 {
		return res
	}

	for _, action := range actions {
		res = append(res, action.convert())
	}
	return res
}

func (actions AttrActionKeyValueSlice) Validate() error {
	var validationErrors []error

	for i, action := range actions {
		if err := action.validate(); err != nil {
			wrappedErr := fmt.Errorf("validation failed for action block number %d: %w", i+1, err)
			validationErrors = append(validationErrors, wrappedErr)
		}
	}

	if len(validationErrors) > 0 {
		return errors.Join(validationErrors...)
	}
	return nil
}

type AttrActionKeyValue struct {
	// Key specifies the attribute to act upon.
	// The actions `delete` and `hash` can use the `pattern`` argument instead of/with the `key` argument.
	// The field is required for all other actions.
	Key string `alloy:"key,attr,optional"`

	// Value specifies the value to populate for the key.
	// The type of the value is inferred from the configuration.
	Value any `alloy:"value,attr,optional"`

	// A regex pattern  must be specified for the action EXTRACT.
	// It uses the attribute specified by `key' to extract values from
	// The target keys are inferred based on the names of the matcher groups
	// provided and the names will be inferred based on the values of the
	// matcher group.
	// Note: All subexpressions must have a name.
	// Note: The value type of the source key must be a string. If it isn't,
	// no extraction will occur.
	RegexPattern string `alloy:"pattern,attr,optional"`

	// FromAttribute specifies the attribute to use to populate
	// the value. If the attribute doesn't exist, no action is performed.
	FromAttribute string `alloy:"from_attribute,attr,optional"`

	// FromContext specifies the context value to use to populate
	// the value. The values would be searched in client.Info.Metadata.
	// If the key doesn't exist, no action is performed.
	// If the key has multiple values the values will be joined with `;` separator.
	FromContext string `alloy:"from_context,attr,optional"`

	// ConvertedType specifies the target type of an attribute to be converted
	// If the key doesn't exist, no action is performed.
	// If the value cannot be converted, the original value will be left as-is
	ConvertedType string `alloy:"converted_type,attr,optional"`

	// Action specifies the type of action to perform.
	// The set of values are {INSERT, UPDATE, UPSERT, DELETE, HASH}.
	// Both lower case and upper case are supported.
	// INSERT -  Inserts the key/value to attributes when the key does not exist.
	//           No action is applied to attributes where the key already exists.
	//           Either Value, FromAttribute or FromContext must be set.
	// UPDATE -  Updates an existing key with a value. No action is applied
	//           to attributes where the key does not exist.
	//           Either Value, FromAttribute or FromContext must be set.
	// UPSERT -  Performs insert or update action depending on the attributes
	//           containing the key. The key/value is inserted to attributes
	//           that did not originally have the key. The key/value is updated
	//           for attributes where the key already existed.
	//           Either Value, FromAttribute or FromContext must be set.
	// DELETE  - Deletes the attribute. If the key doesn't exist,
	//           no action is performed.
	// HASH    - Calculates the SHA-1 hash of an existing value and overwrites the
	//           value with it's SHA-1 hash result.
	// EXTRACT - Extracts values using a regular expression rule from the input
	//           'key' to target keys specified in the 'rule'. If a target key
	//           already exists, it will be overridden.
	// CONVERT  - converts the type of an existing attribute, if convertable
	// This is a required field.
	Action string `alloy:"action,attr"`
}

// Convert converts args into the upstream type.
func (args *AttrActionKeyValue) convert() map[string]any {
	if args == nil {
		return nil
	}

	return map[string]any{
		"key":            args.Key,
		"action":         args.Action,
		"value":          args.Value,
		"pattern":        args.RegexPattern,
		"from_attribute": args.FromAttribute,
		"from_context":   args.FromContext,
		"converted_type": args.ConvertedType,
	}
}

func (args *AttrActionKeyValue) validate() error {
	switch strings.ToLower(args.Action) {
	case delete, hash:
		if args.Key == "" && args.RegexPattern == "" {
			return fmt.Errorf("the action %s requires at least the key argument or the pattern argument to be set", args.Action)
		}
	default:
		if args.Key == "" {
			return fmt.Errorf("the action %s requires the key argument to be set", args.Action)
		}
	}
	return nil
}

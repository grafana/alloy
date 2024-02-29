package otelcol

type AttrActionKeyValueSlice []AttrActionKeyValue

func (actions AttrActionKeyValueSlice) Convert() []interface{} {
	res := make([]interface{}, 0, len(actions))

	if len(actions) == 0 {
		return res
	}

	for _, action := range actions {
		res = append(res, action.convert())
	}
	return res
}

type AttrActionKeyValue struct {
	// Key specifies the attribute to act upon.
	// This is a required field.
	Key string `river:"key,attr"`

	// Value specifies the value to populate for the key.
	// The type of the value is inferred from the configuration.
	Value interface{} `river:"value,attr,optional"`

	// A regex pattern  must be specified for the action EXTRACT.
	// It uses the attribute specified by `key' to extract values from
	// The target keys are inferred based on the names of the matcher groups
	// provided and the names will be inferred based on the values of the
	// matcher group.
	// Note: All subexpressions must have a name.
	// Note: The value type of the source key must be a string. If it isn't,
	// no extraction will occur.
	RegexPattern string `river:"pattern,attr,optional"`

	// FromAttribute specifies the attribute to use to populate
	// the value. If the attribute doesn't exist, no action is performed.
	FromAttribute string `river:"from_attribute,attr,optional"`

	// FromContext specifies the context value to use to populate
	// the value. The values would be searched in client.Info.Metadata.
	// If the key doesn't exist, no action is performed.
	// If the key has multiple values the values will be joined with `;` separator.
	FromContext string `river:"from_context,attr,optional"`

	// ConvertedType specifies the target type of an attribute to be converted
	// If the key doesn't exist, no action is performed.
	// If the value cannot be converted, the original value will be left as-is
	ConvertedType string `river:"converted_type,attr,optional"`

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
	Action string `river:"action,attr"`
}

// Convert converts args into the upstream type.
func (args *AttrActionKeyValue) convert() map[string]interface{} {
	if args == nil {
		return nil
	}

	return map[string]interface{}{
		"key":            args.Key,
		"action":         args.Action,
		"value":          args.Value,
		"pattern":        args.RegexPattern,
		"from_attribute": args.FromAttribute,
		"from_context":   args.FromContext,
		"converted_type": args.ConvertedType,
	}
}

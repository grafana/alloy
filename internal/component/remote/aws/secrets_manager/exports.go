package secrets_manager

import (
	"encoding/json"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	"github.com/grafana/alloy/syntax/alloytypes"
)

var errBinarySecret = errors.New("binary secrets are not supported")

// Exports holds the values that remote.aws.secrets_manager exports.
type Exports struct {
	// Data holds the top-level fields of a secret that is a JSON object.
	Data map[string]alloytypes.Secret `alloy:"data,attr"`
	// Content holds the raw secret string.
	Content alloytypes.Secret `alloy:"content,attr"`
}

func toExports(out *secretsmanager.GetSecretValueOutput) (Exports, error) {
	if out.SecretString == nil {
		return Exports{}, errBinarySecret
	}
	s := *out.SecretString
	return Exports{Data: flatten(s), Content: alloytypes.Secret(s)}, nil
}

// flatten returns the top-level fields of a JSON object. A secret that is not
// a JSON object is valid, so it gives an empty map and no error.
func flatten(s string) map[string]alloytypes.Secret {
	data := map[string]alloytypes.Secret{}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return data
	}
	for k, raw := range obj {
		// Only decode JSON strings. json.Unmarshal of null into a string gives "" and
		// no error, so check the first byte instead of the error.
		if len(raw) > 0 && raw[0] == '"' {
			var str string
			if err := json.Unmarshal(raw, &str); err == nil {
				data[k] = alloytypes.Secret(str)
				continue
			}
		}
		data[k] = alloytypes.Secret(raw)
	}
	return data
}

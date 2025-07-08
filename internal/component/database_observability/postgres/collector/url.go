package collector

import (
	"fmt"
	"strings"

	"github.com/lib/pq"
)

func ParseURL(url string) (map[string]string, error) {
	if url == "postgresql://" || url == "postgres://" {
		return map[string]string{}, nil
	}

	raw, err := pq.ParseURL(url)
	if err != nil {
		return nil, err
	}

	res := map[string]string{}

	unescaper := strings.NewReplacer(`\'`, `'`, `\\`, `\`)

	for keypair := range strings.SplitSeq(raw, " ") {
		parts := strings.SplitN(keypair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected keypair %s from pq", keypair)
		}

		key := parts[0]
		value := parts[1]

		// Undo all the transformations ParseURL did: remove wrapping
		// quotes and then unescape the escaped characters.
		value = strings.TrimPrefix(value, "'")
		value = strings.TrimSuffix(value, "'")
		value = unescaper.Replace(value)

		res[key] = value
	}

	return res, nil
}

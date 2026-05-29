package cfg

import (
	"errors"
	"flag"

	"github.com/grafana/dskit/flagext"
)

// Defaults registers flags to the flagSet using dst as the flagext.Registerer
func Defaults(fs *flag.FlagSet) Source {
	return func(dst Cloneable) error {
		r, ok := dst.(flagext.Registerer)
		if !ok {
			return errors.New("dst does not satisfy flagext.Registerer")
		}

		// already sets the defaults on r
		r.RegisterFlags(fs)
		return nil
	}
}

package cfg

import (
	"flag"
	"testing"
	"time"

	"github.com/grafana/dskit/flagext"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaults checks that defaults are correctly obtained from a
// flagext.Registerer
func TestDefaults(t *testing.T) {
	data := Data{}
	fs := flag.NewFlagSet(t.Name(), flag.PanicOnError)

	err := Unmarshal(&data,
		Defaults(fs),
	)

	require.NoError(t, err)
	assert.Equal(t, Data{
		Verbose: false,
		Server: Server{
			Port:    80,
			Timeout: 60 * time.Second,
		},
		TLS: TLS{
			Cert: "DEFAULTCERT",
			Key:  "DEFAULTKEY",
		},
	}, data)
}

// Data is a test Data structure
type Data struct {
	Verbose bool   `yaml:"verbose"`
	Server  Server `yaml:"server"`
	TLS     TLS    `yaml:"tls"`
}

// Clone takes advantage of pass-by-value semantics to return a distinct *Data.
// This is primarily used to parse a different flag set without mutating the original *Data.
func (d *Data) Clone() flagext.Registerer {
	return func(d Data) *Data {
		return &d
	}(*d)
}

type Server struct {
	Port    int           `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type TLS struct {
	Cert string `yaml:"cert"`
	Key  string `yaml:"key"`
}

// RegisterFlags makes Data implement flagext.Registerer for using flags
func (d *Data) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(&d.Verbose, "verbose", false, "")
	fs.IntVar(&d.Server.Port, "server.port", 80, "")
	fs.DurationVar(&d.Server.Timeout, "server.timeout", 60*time.Second, "")

	fs.StringVar(&d.TLS.Cert, "tls.cert", "DEFAULTCERT", "")
	fs.StringVar(&d.TLS.Key, "tls.key", "DEFAULTKEY", "")
}

package otelcol

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
)

// TLSServerArguments holds shared TLS settings for components which launch
// servers with TLS.
type TLSServerArguments struct {
	TLSSetting TLSSetting `alloy:",squash"`

	ClientCAFile string `alloy:"client_ca_file,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *TLSServerArguments) Convert() configoptional.Optional[configtls.ServerConfig] {
	if args == nil {
		return configoptional.None[configtls.ServerConfig]()
	}

	return configoptional.Some(configtls.ServerConfig{
		Config:       *args.TLSSetting.Convert(),
		ClientCAFile: args.ClientCAFile,
	})
}

// TLSClientArguments holds shared TLS settings for components which launch
// TLS clients.
type TLSClientArguments struct {
	TLSSetting TLSSetting `alloy:",squash"`

	Insecure           bool   `alloy:"insecure,attr,optional"`
	InsecureSkipVerify bool   `alloy:"insecure_skip_verify,attr,optional"`
	ServerName         string `alloy:"server_name,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *TLSClientArguments) Convert() *configtls.ClientConfig {
	if args == nil {
		return nil
	}

	return &configtls.ClientConfig{
		Config:             *args.TLSSetting.Convert(),
		Insecure:           args.Insecure,
		InsecureSkipVerify: args.InsecureSkipVerify,
		ServerName:         args.ServerName,
	}
}

type TLSSetting struct {
	CA                       string            `alloy:"ca_pem,attr,optional"`
	CAFile                   string            `alloy:"ca_file,attr,optional"`
	Cert                     string            `alloy:"cert_pem,attr,optional"`
	CertFile                 string            `alloy:"cert_file,attr,optional"`
	Key                      alloytypes.Secret `alloy:"key_pem,attr,optional"`
	KeyFile                  string            `alloy:"key_file,attr,optional"`
	MinVersion               string            `alloy:"min_version,attr,optional"`
	MaxVersion               string            `alloy:"max_version,attr,optional"`
	ReloadInterval           time.Duration     `alloy:"reload_interval,attr,optional"`
	CipherSuites             []string          `alloy:"cipher_suites,attr,optional"`
	IncludeSystemCACertsPool bool              `alloy:"include_system_ca_certs_pool,attr,optional"`
	CurvePreferences         []string          `alloy:"curve_preferences,attr,optional"`
	TPMConfig                *TPMConfig        `alloy:"tpm,block,optional"`
}

func (args *TLSSetting) Convert() *configtls.Config {
	if args == nil {
		return nil
	}

	t := &configtls.Config{
		CAPem:                    configopaque.String(args.CA),
		CAFile:                   args.CAFile,
		CertPem:                  configopaque.String(args.Cert),
		CertFile:                 args.CertFile,
		KeyPem:                   configopaque.String(string(args.Key)),
		KeyFile:                  args.KeyFile,
		MinVersion:               args.MinVersion,
		MaxVersion:               args.MaxVersion,
		ReloadInterval:           args.ReloadInterval,
		CipherSuites:             args.CipherSuites,
		IncludeSystemCACertsPool: args.IncludeSystemCACertsPool,
		CurvePreferences:         args.CurvePreferences,
		TPMConfig:                args.TPMConfig.Convert(),
	}
	return t
}

// Validate implements syntax.Validator.
func (t *TLSSetting) Validate() error {
	if len(t.CA) > 0 && len(t.CAFile) > 0 {
		return fmt.Errorf("at most one of ca_pem and ca_file must be configured")
	}
	if len(t.Cert) > 0 && len(t.CertFile) > 0 {
		return fmt.Errorf("at most one of cert_pem and cert_file must be configured")
	}
	if len(t.Key) > 0 && len(t.KeyFile) > 0 {
		return fmt.Errorf("at most one of key_pem and key_file must be configured")
	}

	var (
		usingClientCert = len(t.Cert) > 0 || len(t.CertFile) > 0
		usingClientKey  = len(t.Key) > 0 || len(t.KeyFile) > 0
	)

	if usingClientCert && !usingClientKey {
		return fmt.Errorf("exactly one of key_pem or key_file must be configured when a client certificate is configured")
	} else if usingClientKey && !usingClientCert {
		return fmt.Errorf("exactly one of cert_pem or cert_file must be configured when a client key is configured")
	}

	return nil
}

type TPMConfig struct {
	Enabled   bool   `alloy:"enabled,attr,optional"`
	Path      string `alloy:"path,attr,optional"`
	OwnerAuth string `alloy:"owner_auth,attr,optional"`
	Auth      string `alloy:"auth,attr,optional"`
}

func (t *TPMConfig) Convert() configtls.TPMConfig {
	if t == nil {
		return configtls.TPMConfig{}
	}

	return configtls.TPMConfig{
		Enabled:   t.Enabled,
		Path:      t.Path,
		OwnerAuth: t.OwnerAuth,
		Auth:      t.Auth,
	}
}

package otelcol

import (
	"fmt"
	"time"

	"github.com/grafana/river/rivertypes"
	"go.opentelemetry.io/collector/config/configopaque"
	otelconfigtls "go.opentelemetry.io/collector/config/configtls"
)

// TLSServerArguments holds shared TLS settings for components which launch
// servers with TLS.
type TLSServerArguments struct {
	TLSSetting TLSSetting `river:",squash"`

	ClientCAFile string `river:"client_ca_file,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *TLSServerArguments) Convert() *otelconfigtls.TLSServerSetting {
	if args == nil {
		return nil
	}

	return &otelconfigtls.TLSServerSetting{
		TLSSetting:   *args.TLSSetting.Convert(),
		ClientCAFile: args.ClientCAFile,
	}
}

// TLSClientArguments holds shared TLS settings for components which launch
// TLS clients.
type TLSClientArguments struct {
	TLSSetting TLSSetting `river:",squash"`

	Insecure           bool   `river:"insecure,attr,optional"`
	InsecureSkipVerify bool   `river:"insecure_skip_verify,attr,optional"`
	ServerName         string `river:"server_name,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *TLSClientArguments) Convert() *otelconfigtls.TLSClientSetting {
	if args == nil {
		return nil
	}

	return &otelconfigtls.TLSClientSetting{
		TLSSetting:         *args.TLSSetting.Convert(),
		Insecure:           args.Insecure,
		InsecureSkipVerify: args.InsecureSkipVerify,
		ServerName:         args.ServerName,
	}
}

type TLSSetting struct {
	CA             string            `river:"ca_pem,attr,optional"`
	CAFile         string            `river:"ca_file,attr,optional"`
	Cert           string            `river:"cert_pem,attr,optional"`
	CertFile       string            `river:"cert_file,attr,optional"`
	Key            rivertypes.Secret `river:"key_pem,attr,optional"`
	KeyFile        string            `river:"key_file,attr,optional"`
	MinVersion     string            `river:"min_version,attr,optional"`
	MaxVersion     string            `river:"max_version,attr,optional"`
	ReloadInterval time.Duration     `river:"reload_interval,attr,optional"`
}

func (args *TLSSetting) Convert() *otelconfigtls.TLSSetting {
	if args == nil {
		return nil
	}

	return &otelconfigtls.TLSSetting{
		CAPem:          configopaque.String(args.CA),
		CAFile:         args.CAFile,
		CertPem:        configopaque.String(args.Cert),
		CertFile:       args.CertFile,
		KeyPem:         configopaque.String(string(args.Key)),
		KeyFile:        args.KeyFile,
		MinVersion:     args.MinVersion,
		MaxVersion:     args.MaxVersion,
		ReloadInterval: args.ReloadInterval,
	}
}

// Validate implements river.Validator.
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

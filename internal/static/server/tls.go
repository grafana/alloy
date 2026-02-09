package server

import (
	"crypto/tls"
	"errors"
	"fmt"
	"time"
)

// TLSConfig holds dynamic configuration options for TLS.
type TLSConfig struct {
	TLSCertPath              string                    `yaml:"cert_file,omitempty"`
	TLSKeyPath               string                    `yaml:"key_file,omitempty"`
	ClientAuth               string                    `yaml:"client_auth_type,omitempty"`
	ClientCAs                string                    `yaml:"client_ca_file,omitempty"`
	CipherSuites             []TLSCipher               `yaml:"cipher_suites,omitempty"`
	CurvePreferences         []TLSCurve                `yaml:"curve_preferences,omitempty"`
	MinVersion               TLSVersion                `yaml:"min_version,omitempty"`
	MaxVersion               TLSVersion                `yaml:"max_version,omitempty"`
	PreferServerCipherSuites bool                      `yaml:"prefer_server_cipher_suites,omitempty"`
	WindowsCertificateFilter *WindowsCertificateFilter `yaml:"windows_certificate_filter,omitempty"`
}

// WindowsCertificateFilter represents the configuration for accessing the Windows store
type WindowsCertificateFilter struct {
	Server *WindowsServerFilter `yaml:"server,omitempty"`
	Client *WindowsClientFilter `yaml:"client,omitempty"`
}

// WindowsClientFilter is used to select a client root CA certificate
type WindowsClientFilter struct {
	IssuerCommonNames []string `yaml:"issuer_common_names,omitempty"`
	SubjectRegEx      string   `yaml:"subject_regex,omitempty"`
	TemplateID        string   `yaml:"template_id,omitempty"`
}

// WindowsServerFilter is used to select a server certificate
type WindowsServerFilter struct {
	Store             string   `yaml:"store,omitempty"`
	SystemStore       string   `yaml:"system_store,omitempty"`
	IssuerCommonNames []string `yaml:"issuer_common_names,omitempty"`
	TemplateID        string   `yaml:"template_id,omitempty"`

	RefreshInterval time.Duration `yaml:"refresh_interval,omitempty"`
}

// TLSCipher holds the ID of a tls.CipherSuite.
type TLSCipher uint16

// UnmarshalYAML unmarshals the name of a cipher suite to its ID.
func (c *TLSCipher) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	for _, cs := range tls.CipherSuites() {
		if cs.Name == s {
			*c = (TLSCipher)(cs.ID)
			return nil
		}
	}
	return errors.New("unknown cipher: " + s)
}

// MarshalYAML marshals the name of the cipher suite.
func (c TLSCipher) MarshalYAML() (any, error) {
	return tls.CipherSuiteName((uint16)(c)), nil
}

// TLSCurve holds the ID of a TLS elliptic curve.
type TLSCurve tls.CurveID

var curves = map[string]TLSCurve{
	"CurveP256": (TLSCurve)(tls.CurveP256),
	"CurveP384": (TLSCurve)(tls.CurveP384),
	"CurveP521": (TLSCurve)(tls.CurveP521),
	"X25519":    (TLSCurve)(tls.X25519),
}

// UnmarshalYAML unmarshals the name of a TLS elliptic curve into its ID.
func (c *TLSCurve) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	if curveid, ok := curves[s]; ok {
		*c = curveid
		return nil
	}
	return errors.New("unknown curve: " + s)
}

// MarshalYAML marshals the ID of a TLS elliptic curve into its name.
func (c *TLSCurve) MarshalYAML() (any, error) {
	for s, curveid := range curves {
		if *c == curveid {
			return s, nil
		}
	}
	return fmt.Sprintf("%v", c), nil
}

// TLSVersion holds a TLS version ID.
type TLSVersion uint16

var tlsVersions = map[string]TLSVersion{
	"TLS13": (TLSVersion)(tls.VersionTLS13),
	"TLS12": (TLSVersion)(tls.VersionTLS12),
	"TLS11": (TLSVersion)(tls.VersionTLS11),
	"TLS10": (TLSVersion)(tls.VersionTLS10),
}

// UnmarshalYAML unmarshals the name of a TLS version into its ID.
func (tv *TLSVersion) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	if v, ok := tlsVersions[s]; ok {
		*tv = v
		return nil
	}
	return errors.New("unknown TLS version: " + s)
}

// MarshalYAML marshals the ID of a TLS version into its name.
func (tv *TLSVersion) MarshalYAML() (any, error) {
	for s, v := range tlsVersions {
		if *tv == v {
			return s, nil
		}
	}
	return fmt.Sprintf("%v", tv), nil
}

func GetClientAuthFromString(clientAuth string) (tls.ClientAuthType, error) {
	switch clientAuth {
	case "RequestClientCert":
		return tls.RequestClientCert, nil
	case "RequireAnyClientCert", "RequireClientCert": // Preserved for backwards compatibility.
		return tls.RequireAnyClientCert, nil
	case "VerifyClientCertIfGiven":
		return tls.VerifyClientCertIfGiven, nil
	case "RequireAndVerifyClientCert":
		return tls.RequireAndVerifyClientCert, nil
	case "", "NoClientCert":
		return tls.NoClientCert, nil
	default:
		return tls.NoClientCert, fmt.Errorf("invalid ClientAuth %q", clientAuth)
	}
}

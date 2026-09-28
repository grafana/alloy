package otelcol

import (
	"log/slog"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// KafkaAuthenticationArguments configures how to authenticate to the Kafka broker.
type KafkaAuthenticationArguments struct {
	Plaintext *KafkaPlaintextArguments `alloy:"plaintext,block,optional"`
	SASL      *KafkaSASLArguments      `alloy:"sasl,block,optional"`
	TLS       *TLSClientArguments      `alloy:"tls,block,optional"` // Deprecated: no-op, upstream nests TLS under the client config, not under auth. Use the component's top-level tls block instead.
	Kerberos  *KafkaKerberosArguments  `alloy:"kerberos,block,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaAuthenticationArguments) Convert() map[string]any {
	auth := make(map[string]any)

	if args.Plaintext != nil {
		conv := args.Plaintext.Convert()
		auth["sasl"] = &conv
	}
	if args.SASL != nil {
		conv := args.SASL.Convert()
		auth["sasl"] = &conv
	}
	if args.Kerberos != nil {
		conv := args.Kerberos.Convert()
		auth["kerberos"] = &conv
	}

	return auth
}

var _ DeprecationLogger = KafkaAuthenticationArguments{}

// LogDeprecations logs a warning for each deprecated authentication setting in use.
// Call it from the embedding component's own LogDeprecations rather than relying on
// Go method promotion through embedding, since the "authentication." prefix
// assumes the caller's schema places this under an `authentication` block.
func (args KafkaAuthenticationArguments) LogDeprecations(logger *slog.Logger) {
	if logger == nil {
		return
	}
	if args.Plaintext != nil {
		logger.Warn(`authentication.plaintext is deprecated, use authentication.sasl with mechanism set to "PLAIN" instead`)
	}
	if args.SASL != nil && args.SASL.Version != 0 {
		logger.Warn("authentication.sasl.version is deprecated and is a no-op upstream", "version", args.SASL.Version)
	}
	if args.TLS != nil {
		logger.Warn("authentication.tls is deprecated and has no effect; configure the component's top-level tls block instead")
	}
}

// KafkaPlaintextArguments configures plaintext authentication against the Kafka
// broker.
type KafkaPlaintextArguments struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

// Convert converts args into the upstream type. Upstream removed direct plaintext
// authentication after the franz-go migration; SASL with the "PLAIN" mechanism replaces it.
func (args KafkaPlaintextArguments) Convert() map[string]any {
	return map[string]any{
		"username":  args.Username,
		"password":  string(args.Password),
		"mechanism": "PLAIN",
	}
}

// KafkaSASLArguments configures SASL authentication against the Kafka broker.
type KafkaSASLArguments struct {
	Username  string               `alloy:"username,attr"`
	Password  alloytypes.Secret    `alloy:"password,attr"`
	Mechanism string               `alloy:"mechanism,attr"`
	Version   int                  `alloy:"version,attr,optional"` // Deprecated: no-op, upstream removed the SASL handshake version override after the franz-go migration.
	AWSMSK    KafkaAWSMSKArguments `alloy:"aws_msk,block,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaSASLArguments) Convert() map[string]any {
	return map[string]any{
		"username":  args.Username,
		"password":  string(args.Password),
		"mechanism": args.Mechanism,
		"aws_msk":   args.AWSMSK.Convert(),
	}
}

// KafkaAWSMSKArguments exposes additional SASL authentication measures required to
// use the AWS_MSK_IAM_OAUTHBEARER mechanism.
type KafkaAWSMSKArguments struct {
	Region string `alloy:"region,attr"`
}

// Convert converts args into the upstream type.
func (args KafkaAWSMSKArguments) Convert() map[string]any {
	return map[string]any{
		"region": args.Region,
	}
}

// KafkaKerberosArguments configures Kerberos authentication against the Kafka
// broker.
type KafkaKerberosArguments struct {
	ServiceName     string            `alloy:"service_name,attr,optional"`
	Realm           string            `alloy:"realm,attr,optional"`
	UseKeyTab       bool              `alloy:"use_keytab,attr,optional"`
	Username        string            `alloy:"username,attr"`
	Password        alloytypes.Secret `alloy:"password,attr,optional"`
	ConfigPath      string            `alloy:"config_file,attr,optional"`
	KeyTabPath      string            `alloy:"keytab_file,attr,optional"`
	DisablePAFXFAST bool              `alloy:"disable_fast_negotiation,attr,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaKerberosArguments) Convert() map[string]any {
	return map[string]any{
		"service_name":             args.ServiceName,
		"realm":                    args.Realm,
		"use_keytab":               args.UseKeyTab,
		"username":                 args.Username,
		"password":                 string(args.Password),
		"config_file":              args.ConfigPath,
		"keytab_file":              args.KeyTabPath,
		"disable_fast_negotiation": args.DisablePAFXFAST,
	}
}

// KafkaMetadataArguments configures how the Alloy component will
// retrieve metadata from the Kafka broker.
type KafkaMetadataArguments struct {
	Full            bool                        `alloy:"full,attr,optional"`
	RefreshInterval time.Duration               `alloy:"refresh_interval,attr,optional"`
	Retry           KafkaMetadataRetryArguments `alloy:"retry,block,optional"`
}

func (args *KafkaMetadataArguments) SetToDefault() {
	*args = KafkaMetadataArguments{
		Full:            true,
		RefreshInterval: 10 * time.Minute,
		Retry: KafkaMetadataRetryArguments{
			MaxRetries: 3,
			Backoff:    250 * time.Millisecond,
		},
	}
}

// Convert converts args into the upstream type.
func (args KafkaMetadataArguments) Convert() configkafka.MetadataConfig {
	return configkafka.MetadataConfig{
		Full:            args.Full,
		RefreshInterval: args.RefreshInterval,
		Retry:           args.Retry.Convert(),
	}
}

// KafkaMetadataRetryArguments configures how to retry retrieving metadata from the
// Kafka broker. Retrying is useful to avoid race conditions when the Kafka
// broker is starting at the same time as the Alloy component.
type KafkaMetadataRetryArguments struct {
	MaxRetries int           `alloy:"max_retries,attr,optional"`
	Backoff    time.Duration `alloy:"backoff,attr,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaMetadataRetryArguments) Convert() configkafka.MetadataRetryConfig {
	return configkafka.MetadataRetryConfig{
		Max:     args.MaxRetries,
		Backoff: args.Backoff,
	}
}

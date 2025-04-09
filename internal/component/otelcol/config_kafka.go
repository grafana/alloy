package otelcol

import (
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
)

// KafkaAuthenticationArguments configures how to authenticate to the Kafka broker.
type KafkaAuthenticationArguments struct {
	Plaintext *KafkaPlaintextArguments `alloy:"plaintext,block,optional"`
	SASL      *KafkaSASLArguments      `alloy:"sasl,block,optional"`
	TLS       *TLSClientArguments      `alloy:"tls,block,optional"`
	Kerberos  *KafkaKerberosArguments  `alloy:"kerberos,block,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaAuthenticationArguments) Convert() map[string]interface{} {
	auth := make(map[string]interface{})

	if args.Plaintext != nil {
		conv := args.Plaintext.Convert()
		auth["plain_text"] = &conv
	}
	if args.SASL != nil {
		conv := args.SASL.Convert()
		auth["sasl"] = &conv
	}
	if args.TLS != nil {
		auth["tls"] = args.TLS.Convert()
	}
	if args.Kerberos != nil {
		conv := args.Kerberos.Convert()
		auth["kerberos"] = &conv
	}

	return auth
}

// KafkaPlaintextArguments configures plaintext authentication against the Kafka
// broker.
type KafkaPlaintextArguments struct {
	Username string            `alloy:"username,attr"`
	Password alloytypes.Secret `alloy:"password,attr"`
}

// Convert converts args into the upstream type.
func (args KafkaPlaintextArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"username": args.Username,
		"password": string(args.Password),
	}
}

// KafkaSASLArguments configures SASL authentication against the Kafka broker.
type KafkaSASLArguments struct {
	Username  string               `alloy:"username,attr"`
	Password  alloytypes.Secret    `alloy:"password,attr"`
	Mechanism string               `alloy:"mechanism,attr"`
	Version   int                  `alloy:"version,attr,optional"`
	AWSMSK    KafkaAWSMSKArguments `alloy:"aws_msk,block,optional"`
}

// Convert converts args into the upstream type.
func (args KafkaSASLArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"username":  args.Username,
		"password":  string(args.Password),
		"mechanism": args.Mechanism,
		"version":   args.Version,
		"aws_msk":   args.AWSMSK.Convert(),
	}
}

// KafkaAWSMSKArguments exposes additional SASL authentication measures required to
// use the AWS_MSK_IAM mechanism.
type KafkaAWSMSKArguments struct {
	Region     string `alloy:"region,attr"`
	BrokerAddr string `alloy:"broker_addr,attr"`
}

// Convert converts args into the upstream type.
func (args KafkaAWSMSKArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
		"region":      args.Region,
		"broker_addr": args.BrokerAddr,
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
func (args KafkaKerberosArguments) Convert() map[string]interface{} {
	return map[string]interface{}{
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
	IncludeAllTopics bool                        `alloy:"include_all_topics,attr,optional"`
	Retry            KafkaMetadataRetryArguments `alloy:"retry,block,optional"`
}

func (args *KafkaMetadataArguments) SetToDefault() {
	*args = KafkaMetadataArguments{
		IncludeAllTopics: true,
		Retry: KafkaMetadataRetryArguments{
			MaxRetries: 3,
			Backoff:    250 * time.Millisecond,
		},
	}
}

// Convert converts args into the upstream type.
func (args KafkaMetadataArguments) Convert() kafkaexporter.Metadata {
	return kafkaexporter.Metadata{
		Full:  args.IncludeAllTopics,
		Retry: args.Retry.Convert(),
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
func (args KafkaMetadataRetryArguments) Convert() kafkaexporter.MetadataRetry {
	return kafkaexporter.MetadataRetry{
		Max:     args.MaxRetries,
		Backoff: args.Backoff,
	}
}

// Package kafkaclient holds the Kafka client configuration and record format
// shared by the kafka.tenant_* components.
package kafkaclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	promconfig "github.com/prometheus/common/config"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/kafka/tenant/registry"
	"github.com/grafana/alloy/syntax/alloytypes"
)

// Record headers written by kafka.tenant_producer and read by
// kafka.tenant_consumer.
const (
	HeaderTenantID        = "tenant_id"
	HeaderSignal          = "signal"
	HeaderFormat          = "format"
	HeaderContentType     = "content_type"
	HeaderContentEncoding = "content_encoding"
	HeaderURL             = "url"
	HeaderSchemaVersion   = "schema_version"

	SchemaVersion = "1"
)

// Signal values for HeaderSignal.
const (
	SignalMetrics  = "metrics"
	SignalLogs     = "logs"
	SignalTraces   = "traces"
	SignalProfiles = "profiles"
)

// Format values for HeaderFormat.
const (
	FormatPromRWv1        = "prom_rw_v1"
	FormatLokiPush        = "loki_push"
	FormatPyroscopeIngest = "pyroscope_ingest"
	FormatOTLP            = "otlp"
)

// Header returns the value of the first record header named key.
func Header(r *kgo.Record, key string) string {
	for _, h := range r.Headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

// Arguments configures the connection to the Kafka (WarpStream) cluster.
type Arguments struct {
	Brokers  []string          `alloy:"brokers,attr"`
	ClientID string            `alloy:"client_id,attr,optional"`
	TLS      *config.TLSConfig `alloy:"tls,block,optional"`
	SASL     *SASLArguments    `alloy:"sasl,block,optional"`
}

// SASLArguments configures SASL authentication.
type SASLArguments struct {
	Mechanism string            `alloy:"mechanism,attr,optional"`
	Username  string            `alloy:"username,attr"`
	Password  alloytypes.Secret `alloy:"password,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (s *SASLArguments) SetToDefault() {
	*s = SASLArguments{Mechanism: "PLAIN"}
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.Brokers) == 0 {
		return errors.New("at least one broker must be set")
	}
	if a.SASL != nil {
		switch strings.ToUpper(a.SASL.Mechanism) {
		case "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512":
		default:
			return fmt.Errorf("unsupported SASL mechanism %q: valid values are PLAIN, SCRAM-SHA-256, SCRAM-SHA-512", a.SASL.Mechanism)
		}
	}
	return nil
}

// Opts returns the franz-go options for connecting to the cluster.
func (a Arguments) Opts() ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(a.Brokers...)}
	if a.ClientID != "" {
		opts = append(opts, kgo.ClientID(a.ClientID))
	}

	if a.TLS != nil {
		tlsCfg, err := promconfig.NewTLSConfig(a.TLS.Convert())
		if err != nil {
			return nil, fmt.Errorf("building TLS config: %w", err)
		}
		opts = append(opts, kgo.DialTLSConfig(tlsCfg))
	}

	if a.SASL != nil {
		user, pass := a.SASL.Username, string(a.SASL.Password)
		switch strings.ToUpper(a.SASL.Mechanism) {
		case "PLAIN":
			opts = append(opts, kgo.SASL(plain.Auth{User: user, Pass: pass}.AsMechanism()))
		case "SCRAM-SHA-256":
			opts = append(opts, kgo.SASL(scram.Auth{User: user, Pass: pass}.AsSha256Mechanism()))
		case "SCRAM-SHA-512":
			opts = append(opts, kgo.SASL(scram.Auth{User: user, Pass: pass}.AsSha512Mechanism()))
		}
	}

	return opts, nil
}

// CheckTopic verifies that the registry's topic exists and has at least as
// many partitions as the registry declares. Components never create topics.
func CheckTopic(ctx context.Context, cl *kgo.Client, reg *registry.Registry) error {
	topics, err := kadm.NewClient(cl).ListTopics(ctx, reg.Topic())
	if err != nil {
		return fmt.Errorf("listing topic %q: %w", reg.Topic(), err)
	}
	td, ok := topics[reg.Topic()]
	if !ok || errors.Is(td.Err, kerr.UnknownTopicOrPartition) {
		return fmt.Errorf("topic %q does not exist; it must be created before starting the component", reg.Topic())
	}
	if td.Err != nil {
		return fmt.Errorf("topic %q: %w", reg.Topic(), td.Err)
	}
	if n := int32(len(td.Partitions)); n < reg.Partitions() {
		return fmt.Errorf("topic %q has %d partitions but the tenant registry declares %d", reg.Topic(), n, reg.Partitions())
	}
	return nil
}

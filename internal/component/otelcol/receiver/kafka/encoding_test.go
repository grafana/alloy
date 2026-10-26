package kafka_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

type nopExtension struct {
	otelcomponent.StartFunc
	otelcomponent.ShutdownFunc
}

// evaluateWithHandlers evaluates cfg with otelcol.encoding.azure.<label>.handler
// in scope for every handler in handlers.
func evaluateWithHandlers(t *testing.T, cfg string, handlers map[string]*extension.ExtensionHandler) (kafka.Arguments, error) {
	t.Helper()

	azure := make(map[string]any, len(handlers))
	for label, h := range handlers {
		azure[label] = extension.Exports{Handler: h}
	}
	scope := vm.NewScope(map[string]any{
		"otelcol": map[string]any{
			"encoding": map[string]any{"azure": azure},
		},
	})

	file, err := parser.ParseFile(t.Name(), []byte(cfg))
	require.NoError(t, err)

	var args kafka.Arguments
	err = vm.New(file).Evaluate(scope, &args)
	return args, err
}

func newHandler(name string) *extension.ExtensionHandler {
	return &extension.ExtensionHandler{
		ID:        otelcomponent.MustNewIDWithName("azure_encoding", name),
		Extension: nopExtension{},
	}
}

func TestArguments_EncodingMapping(t *testing.T) {
	handlers := map[string]*extension.ExtensionHandler{
		"default": newHandler("otelcol.encoding.azure.default"),
	}

	tests := []struct {
		name     string
		signal   string
		expected string
	}{
		{
			name:     "both unset uses upstream default",
			signal:   ``,
			expected: "otlp_proto",
		},
		{
			name:     "explicit encoding is passed through",
			signal:   `encoding = "json"`,
			expected: "json",
		},
		{
			name:     "unknown encoding is left to upstream",
			signal:   `encoding = "bogus"`,
			expected: "bogus",
		},
		{
			name:     "extension uses handler ID",
			signal:   `encoding_extension = otelcol.encoding.azure.default.handler`,
			expected: "azure_encoding/otelcol.encoding.azure.default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args, err := evaluateWithHandlers(t, `
				brokers          = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"
				logs {
					`+tc.signal+`
				}
				output {}
			`, handlers)
			require.NoError(t, err)

			actual, err := args.Convert()
			require.NoError(t, err)

			cfg := actual.(*kafkareceiver.Config)
			require.Equal(t, tc.expected, cfg.Logs.Encoding)
			require.Equal(t, "otlp_proto", cfg.Metrics.Encoding)
			require.Equal(t, "otlp_proto", cfg.Traces.Encoding)
		})
	}
}

func TestArguments_EncodingAndExtensionAreMutuallyExclusive(t *testing.T) {
	handlers := map[string]*extension.ExtensionHandler{
		"default": newHandler("otelcol.encoding.azure.default"),
	}

	// Explicitly setting the default value must also be rejected.
	for _, encoding := range []string{"json", "otlp_proto"} {
		t.Run(encoding, func(t *testing.T) {
			_, err := evaluateWithHandlers(t, `
				brokers          = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"
				traces {
					encoding           = "`+encoding+`"
					encoding_extension = otelcol.encoding.azure.default.handler
				}
				output {}
			`, handlers)
			require.ErrorContains(t, err, "traces: encoding and encoding_extension are mutually exclusive")
		})
	}
}

func TestArguments_EncodingExtensions(t *testing.T) {
	logsHandler := newHandler("otelcol.encoding.azure.logs")
	sharedHandler := newHandler("otelcol.encoding.azure.shared")

	t.Run("none", func(t *testing.T) {
		var args kafka.Arguments
		args.SetToDefault()
		require.Nil(t, args.Extensions())
	})

	t.Run("deduplicated by ID", func(t *testing.T) {
		args := kafka.Arguments{
			Logs:    kafka.KafkaReceiverTopicEncodingConfig{EncodingExtension: logsHandler},
			Metrics: kafka.KafkaReceiverTopicEncodingConfig{EncodingExtension: sharedHandler},
			Traces:  kafka.KafkaReceiverTopicEncodingConfig{EncodingExtension: sharedHandler},
		}
		require.Equal(t, map[otelcomponent.ID]otelcomponent.Component{
			logsHandler.ID:   logsHandler.Extension,
			sharedHandler.ID: sharedHandler.Extension,
		}, args.Extensions())
	})

	t.Run("nil extension is skipped", func(t *testing.T) {
		args := kafka.Arguments{
			Logs: kafka.KafkaReceiverTopicEncodingConfig{
				EncodingExtension: &extension.ExtensionHandler{ID: logsHandler.ID},
			},
		}
		require.Nil(t, args.Extensions())
	})
}

// Upstream parses the encoding string back into a component ID to find the
// extension in the host, so Alloy extension IDs must survive the round trip.
func TestEncodingExtensionIDRoundTrip(t *testing.T) {
	for _, name := range []string{
		"otelcol.encoding.azure.default",
		"customComponent.foo/otelcol.encoding.azure.default",
	} {
		t.Run(name, func(t *testing.T) {
			id := otelcomponent.MustNewIDWithName("azure_encoding", name)

			var parsed otelcomponent.ID
			require.NoError(t, parsed.UnmarshalText([]byte(id.String())))
			require.Equal(t, id, parsed)
		})
	}
}

type extensionsHost map[otelcomponent.ID]otelcomponent.Component

func (h extensionsHost) GetExtensions() map[otelcomponent.ID]otelcomponent.Component { return h }

// TestReceiverResolvesEncodingExtension starts the upstream receiver with the
// converted config and the host extensions Alloy provides, proving the
// receiver picks up the Azure encoding extension. No broker is needed: the
// encoding is resolved during Start, before any records are fetched.
func TestReceiverResolvesEncodingExtension(t *testing.T) {
	azureFactory := azureencodingextension.NewFactory()
	azureExt, err := azureFactory.Create(
		t.Context(),
		extensiontest.NewNopSettings(azureFactory.Type()),
		azureFactory.CreateDefaultConfig(),
	)
	require.NoError(t, err)

	handler := &extension.ExtensionHandler{
		ID:        otelcomponent.NewIDWithName(azureFactory.Type(), "otelcol.encoding.azure.default"),
		Extension: azureExt,
	}

	tests := []struct {
		name      string
		handler   *extension.ExtensionHandler
		expectErr string
	}{
		{name: "extension resolved", handler: handler},
		{name: "extension missing from host", handler: &extension.ExtensionHandler{ID: handler.ID}, expectErr: "unrecognized logs encoding"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args kafka.Arguments
			args.SetToDefault()
			args.Brokers = []string{"127.0.0.1:1"}
			args.ProtocolVersion = "2.0.0"
			args.Logs.EncodingExtension = tc.handler

			cfg, err := args.Convert()
			require.NoError(t, err)

			factory := kafkareceiver.NewFactory()
			recv, err := factory.CreateLogs(t.Context(), receivertest.NewNopSettings(factory.Type()), cfg, consumertest.NewNop())
			require.NoError(t, err)

			err = recv.Start(t.Context(), extensionsHost(args.Extensions()))
			if tc.expectErr != "" {
				// Upstream Shutdown blocks forever if Start failed before
				// launching its consume loop, so don't call it here.
				require.ErrorContains(t, err, tc.expectErr)
				return
			}
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			require.NoError(t, recv.Shutdown(ctx))
		})
	}
}

func TestArguments_LogDeprecations_AzureResourceLogs(t *testing.T) {
	encoding := "azure_resource_logs"
	args := kafka.Arguments{
		Logs: kafka.KafkaReceiverTopicEncodingConfig{Encoding: &encoding},
	}

	var buf bytes.Buffer
	args.LogDeprecations(slog.New(slog.NewTextHandler(&buf, nil)))
	require.Contains(t, buf.String(), "otelcol.encoding.azure")
	require.Contains(t, buf.String(), "signal=logs")
}

module github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/oklog/ulid/v2 v2.1.1
	github.com/open-telemetry/opamp-go v0.22.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.142.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status v0.142.0
	github.com/shirou/gopsutil/v4 v4.25.11
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.48.0
	go.opentelemetry.io/collector/component/componentstatus v0.142.0
	go.opentelemetry.io/collector/component/componenttest v0.142.0
	go.opentelemetry.io/collector/config/configopaque v1.48.0
	go.opentelemetry.io/collector/config/configtls v1.48.0
	go.opentelemetry.io/collector/confmap v1.48.0
	go.opentelemetry.io/collector/extension v1.48.0
	go.opentelemetry.io/collector/extension/extensionauth v1.48.0
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.142.0
	go.opentelemetry.io/collector/extension/extensiontest v0.142.0
	go.opentelemetry.io/collector/service v0.142.0
	go.opentelemetry.io/collector/service/hostcapabilities v0.142.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
	google.golang.org/grpc v1.77.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-tpm v0.9.7 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/michel-laterman/proxy-connect-dialer-go v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.1 // indirect
	github.com/prometheus/otlptranslator v0.0.2 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.142.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.142.0 // indirect
	go.opentelemetry.io/collector/connector v0.142.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.142.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.142.0 // indirect
	go.opentelemetry.io/collector/consumer v1.48.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.142.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.142.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.142.0 // indirect
	go.opentelemetry.io/collector/exporter v1.48.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.142.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.142.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.48.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.142.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.142.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.142.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.142.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.142.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.142.0 // indirect
	go.opentelemetry.io/collector/processor v1.48.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.142.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.142.0 // indirect
	go.opentelemetry.io/collector/receiver v1.48.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.142.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.142.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.18.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.60.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.38.0 // indirect
	go.opentelemetry.io/otel/log v0.15.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.14.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.1 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251022142026-3a174f9686a8 // indirect
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/pdata v1.48.0
	go.opentelemetry.io/collector/pipeline v1.48.0
	go.opentelemetry.io/otel v1.39.0
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// BEGIN GENERATED REPLACES - DO NOT EDIT MANUALLY
// Replace yaml.v2 with fork
replace gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc

// Replace directives from Loki — Azure SDK
replace github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v68.0.0+incompatible

// Replace directives from Loki — Azure storage blob fork
replace github.com/Azure/azure-storage-blob-go => github.com/MasslessParticle/azure-storage-blob-go v0.14.1-0.20240322194317-344980fda573

// Use fork of gocql that has gokit logs and Prometheus metrics
replace github.com/gocql/gocql => github.com/grafana/gocql v0.0.0-20200605141915-ba5dc39ece85

// Insist on the optimised version of grafana/regexp
replace github.com/grafana/regexp => github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc

// Replace memberlist with Grafana fork which includes unmerged upstream fixes
replace github.com/hashicorp/memberlist => github.com/grafana/memberlist v0.3.1-0.20220714140823-09ffed8adbbe

// Use forked syslog implementation by leodido for continued support
replace github.com/influxdata/go-syslog/v3 => github.com/leodido/go-syslog/v4 v4.3.0

// Replace thanos-io/objstore with Grafana fork
replace github.com/thanos-io/objstore => github.com/grafana/objstore v0.0.0-20250210100727-533688b5600d

// TODO - remove forks when changes are merged upstream — non-singleton cadvisor
replace github.com/google/cadvisor => github.com/grafana/cadvisor v0.0.0-20260204200106-865a22723970

// TODO - this tracks exporter-package-v0.19.1 branch of grafana fork; remove once all patches are merged upstream
replace github.com/prometheus-community/postgres_exporter => github.com/grafana/postgres_exporter v0.0.0-20260225165717-9c2c77e3702a

// TODO - remove once PR is merged upstream - https://github.com/prometheus/mysqld_exporter/pull/774
replace github.com/prometheus/mysqld_exporter => github.com/grafana/mysqld_exporter v0.17.2-0.20250226152553-be612e3fdedd

// TODO: replace node_exporter with custom fork for multi usage. https://github.com/prometheus/node_exporter/pull/2812
replace github.com/prometheus/node_exporter => github.com/grafana/node_exporter v0.18.1-grafana-r01.0.20251024135609-318b01780c89

// Use Grafana fork of smimesign
replace github.com/github/smimesign => github.com/grafana/smimesign v0.2.1-0.20220408144937-2a5adf3481d3

// Replace OpenTelemetry OBI with Grafana fork
replace go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation v1.4.11

// Replace OpenTelemetry eBPF profiler with Grafana fork
replace go.opentelemetry.io/ebpf-profiler => github.com/grafana/opentelemetry-ebpf-profiler v0.0.202602-0.20260216144214-241376220646

// Update openshift/client-go to version compatible with structured-merge-diff v6
replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235

// Do not remove until bug in walqueue backwards compatibility is resolved: https://github.com/deneonet/benc/issues/13
replace github.com/deneonet/benc => github.com/deneonet/benc v1.1.7

// Pin runc to v1.2.8 for compatibility with cadvisor requiring libcontainer/cgroups packages
replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.2.8

// Replace controller-runtime with pinned version
replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.20.4

// Fork to grafana repo to address issue with freebsd build tags. This can be removed once https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/42645 is fixed
replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver => github.com/grafana/opentelemetry-collector-contrib/receiver/filestatsreceiver v0.0.0-20260126095124-0af81a9e8966

// TODO: Fork to update Prometheus to v0.309.1 while keeping OTel Collector at v0.142.0. Remove when OTel Collector is upgraded to v0.144.0+, which includes this change upstream.
replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => github.com/grafana/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.0.0-20260209185749-2202e1443a98

// Fix sent_batch_duration_seconds measuring before the request was sent. Fork branch: https://github.com/grafana/prometheus/tree/fix-sent-batch-duration-v0.309.1 Remove when https://github.com/prometheus/prometheus/pull/18214 is merged and Prometheus is upgraded.
replace github.com/prometheus/prometheus => github.com/grafana/prometheus v1.8.2-0.20260302171028-8cf60eef5463

// END GENERATED REPLACES

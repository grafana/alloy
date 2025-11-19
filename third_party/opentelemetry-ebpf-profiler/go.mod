module go.opentelemetry.io/ebpf-profiler

// NOTE:
// This go.mod is NOT used to build any official binary.
// To see the builder manifests used for official binaries,
// check https://github.com/open-telemetry/opentelemetry-collector-releases
//
// For the OpenTelemetry eBPF Profiler distribution specifically, see
// https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-ebpf-profiler

go 1.24.0

require (
	github.com/aws/aws-sdk-go-v2 v1.39.4
	github.com/aws/aws-sdk-go-v2/config v1.31.15
	github.com/aws/aws-sdk-go-v2/service/s3 v1.88.7
	github.com/cilium/ebpf v0.19.0
	github.com/elastic/go-freelru v0.16.0
	github.com/elastic/go-perf v0.0.0-20241029065020-30bec95324b8
	github.com/google/uuid v1.6.0
	github.com/klauspost/compress v1.18.1
	github.com/mdlayher/kobject v0.0.0-20200520190114-19ca17470d7d
	github.com/minio/sha256-simd v1.0.1
	github.com/peterbourgon/ff/v3 v3.4.0
	github.com/sirupsen/logrus v1.9.3
	github.com/stretchr/testify v1.11.1
	github.com/zeebo/xxh3 v1.0.2
	go.opentelemetry.io/collector/component v1.44.0
	go.opentelemetry.io/collector/consumer/consumertest v0.138.0
	go.opentelemetry.io/collector/consumer/xconsumer v0.138.0
	go.opentelemetry.io/collector/pdata v1.44.0
	go.opentelemetry.io/collector/pdata/pprofile v0.138.0
	go.opentelemetry.io/collector/receiver v1.44.0
	go.opentelemetry.io/collector/receiver/receivertest v0.138.0
	go.opentelemetry.io/collector/receiver/xreceiver v0.138.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/metric v1.38.0
	golang.org/x/arch v0.22.0
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546
	golang.org/x/mod v0.29.0
	golang.org/x/sync v0.17.0
	golang.org/x/sys v0.37.0
	google.golang.org/grpc v1.76.0
)

require (
	github.com/grafana/pyroscope/lidia v0.0.0-20250716102313-506840f4afcd
	github.com/prometheus/common v0.55.0
	github.com/prometheus/prometheus v0.51.2
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.2 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.19 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.9 // indirect
	github.com/aws/smithy-go v1.23.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/grafana/regexp v0.0.0-20221122212121-6b5c0a4cb7fd // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/jsimonetti/rtnetlink/v2 v2.0.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.4.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component/componenttest v0.138.0 // indirect
	go.opentelemetry.io/collector/consumer v1.44.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.138.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.44.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.138.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.44.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.13.0 // indirect
	go.opentelemetry.io/otel/log v0.14.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

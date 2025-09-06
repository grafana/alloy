


```go
package test

import (
	"testing"

	"github.com/grafana/alloy/internal/util"
)

func TestName(t *testing.T) {
	_ = util.TestLogger(t).Log("hello", "korniltsev")
}
```

Let's try to compile it.

```bash
~/a/i/test (korniltsev/slimutil-logger) > time go test -c . && ls -ltrh
go: downloading github.com/stretchr/testify v1.10.0
go: downloading github.com/prometheus/client_golang v1.23.0
go: downloading github.com/grafana/dskit v0.0.0-20250703125411-00229f5b510c
go: downloading github.com/go-kit/log v0.2.1
go: downloading github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.128.0
go: downloading github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.128.0
go: downloading go.uber.org/atomic v1.11.0
go: downloading github.com/grafana/opentelemetry-collector/featuregate v0.0.0-20240325174506-2fd1623b2ca0
go: downloading github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc
go: downloading k8s.io/utils v0.0.0-20250604170112-4c0f3b243397
go: downloading github.com/prometheus/common v0.65.1-0.20250804173848-0ad974f9af53
go: downloading github.com/grafana/loki/v3 v3.0.0-20250630063055-0ee8e76ba280
go: downloading github.com/blang/semver/v4 v4.0.0
go: downloading gopkg.in/yaml.v3 v3.0.1
go: downloading github.com/ohler55/ojg v1.26.8
go: downloading github.com/fatih/color v1.18.0
go: downloading github.com/mattn/go-isatty v0.0.20
go: downloading github.com/mattn/go-colorable v0.1.14
go: downloading github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
go: downloading github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
go: downloading golang.org/x/sys v0.35.0
go: downloading go.opentelemetry.io/collector v0.128.0
go: downloading github.com/go-logfmt/logfmt v0.6.0
go: downloading github.com/beorn7/perks v1.0.1
go: downloading github.com/prometheus/procfs v0.17.0
go: downloading github.com/prometheus/client_model v0.6.2
go: downloading github.com/cespare/xxhash/v2 v2.3.0
go: downloading google.golang.org/protobuf v1.36.7
go: downloading github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822
go: downloading go.uber.org/zap v1.27.0
go: downloading github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.128.0
go: downloading go.opentelemetry.io/collector/processor v1.34.0
go: downloading go.opentelemetry.io/collector/component/componentstatus v0.128.0
go: downloading go.opentelemetry.io/collector/component v1.34.0
go: downloading go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.128.0
go: downloading go.opentelemetry.io/collector/consumer/xconsumer v0.128.0
go: downloading go.opentelemetry.io/collector/pdata/pprofile v0.128.0
go: downloading go.opentelemetry.io/collector/processor/processorhelper v0.128.0
go: downloading go.opentelemetry.io/collector/consumer v1.35.0
go: downloading go.opentelemetry.io/otel v1.37.0
go: downloading github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.128.0
go: downloading k8s.io/apimachinery v0.33.2
go: downloading go.opentelemetry.io/collector/pdata v1.35.0
go: downloading go.opentelemetry.io/collector/processor/xprocessor v0.128.0
go: downloading go.opentelemetry.io/otel/metric v1.37.0
go: downloading go.opentelemetry.io/otel/trace v1.37.0
go: downloading github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.128.0
go: downloading go.opentelemetry.io/collector/confmap v1.35.0
go: downloading go.opentelemetry.io/collector/client v1.34.0
go: downloading github.com/distribution/reference v0.6.0
go: downloading k8s.io/api v0.33.2
go: downloading k8s.io/client-go v0.32.6
go: downloading github.com/opencontainers/go-digest v1.0.0
go: downloading go.uber.org/multierr v1.11.0
go: downloading github.com/hashicorp/go-version v1.7.0
go: downloading golang.org/x/text v0.27.0
go: downloading go.opentelemetry.io/collector/pipeline v0.128.0
go: downloading go.opentelemetry.io/collector/extension/xextension v0.128.0
go: downloading github.com/expr-lang/expr v1.17.5
go: downloading github.com/json-iterator/go v1.1.12
go: downloading gonum.org/v1/gonum v0.16.0
go: downloading github.com/bmatcuk/doublestar/v4 v4.9.0
go: downloading github.com/gogo/protobuf v1.3.2
go: downloading google.golang.org/grpc v1.74.2
go: downloading go.opentelemetry.io/collector/internal/telemetry v0.128.0
go: downloading github.com/jonboulle/clockwork v0.5.0
go: downloading github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142
go: downloading github.com/elastic/lunes v0.1.0
go: downloading go.opentelemetry.io/collector/extension v1.34.0
go: downloading sigs.k8s.io/randfill v1.0.0
go: downloading github.com/bmatcuk/doublestar v1.3.4
go: downloading k8s.io/klog/v2 v2.130.1
go: downloading sigs.k8s.io/structured-merge-diff/v4 v4.7.0
go: downloading github.com/go-viper/mapstructure/v2 v2.4.0
go: downloading github.com/gobwas/glob v0.2.3
go: downloading github.com/knadh/koanf v1.5.0
go: downloading github.com/knadh/koanf/v2 v2.2.1
go: downloading sigs.k8s.io/yaml v1.6.0
go: downloading github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
go: downloading github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee
go: downloading gopkg.in/inf.v0 v0.9.1
go: downloading sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8
go: downloading golang.org/x/net v0.42.0
go: downloading github.com/spf13/pflag v1.0.7
go: downloading golang.org/x/term v0.33.0
go: downloading github.com/fxamacker/cbor/v2 v2.8.0
go: downloading github.com/go-logr/logr v1.4.3
go: downloading go.opentelemetry.io/contrib/bridges/otelzap v0.11.0
go: downloading go.opentelemetry.io/otel/log v0.12.2
go: downloading go.opentelemetry.io/otel/sdk v1.37.0
go: downloading github.com/golang/protobuf v1.5.4
go: downloading github.com/google/gnostic-models v0.6.9
go: downloading golang.org/x/time v0.12.0
go: downloading github.com/google/go-cmp v0.7.0
go: downloading golang.org/x/oauth2 v0.30.0
go: downloading google.golang.org/genproto/googleapis/rpc v0.0.0-20250728155136-f173205681a0
go: downloading k8s.io/kube-openapi v0.0.0-20250318190949-c8a335a9a2ff
go: downloading go.yaml.in/yaml/v3 v3.0.3
go: downloading go.yaml.in/yaml/v2 v2.4.2
go: downloading github.com/mitchellh/copystructure v1.2.0
go: downloading gopkg.in/evanphx/json-patch.v4 v4.12.0
go: downloading google.golang.org/genproto v0.0.0-20250603155806-513f23925822
go: downloading go.opentelemetry.io/auto/sdk v1.1.0
go: downloading github.com/go-logr/stdr v1.2.2
go: downloading github.com/google/uuid v1.6.0
go: downloading github.com/pkg/errors v0.9.1
go: downloading github.com/x448/float16 v0.8.4
go: downloading github.com/mitchellh/reflectwalk v1.0.2
go: downloading github.com/go-openapi/swag v0.23.1
go: downloading github.com/go-openapi/jsonreference v0.21.0
go: downloading github.com/emicklei/go-restful/v3 v3.12.2
go: downloading github.com/go-openapi/jsonpointer v0.21.1
go: downloading github.com/mailru/easyjson v0.9.0
go: downloading github.com/josharian/intern v1.0.0
go: downloading github.com/openshift/api v3.9.0+incompatible
go: downloading github.com/grafana/prometheus v1.8.2-0.20250811161144-6e21f656d8e5
go: downloading github.com/c2h5oh/datasize v0.0.0-20231215233829-aa82cc1e6500
go: downloading github.com/grafana/loki/pkg/push v0.0.0-20250630063055-0ee8e76ba280
go: downloading github.com/dustin/go-humanize v1.0.1
go: downloading github.com/gogo/status v1.1.1
go: downloading github.com/gogo/googleapis v1.4.1
go: downloading github.com/Masterminds/sprig/v3 v3.3.0
go: downloading github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc
go: downloading github.com/golang/snappy v1.0.0
go: downloading github.com/grafana/jsonparser v0.0.0-20241004153430-023329977675
go: downloading github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b
go: downloading github.com/tjhop/slog-gokit v0.1.4
go: downloading github.com/gorilla/mux v1.8.1
go: downloading github.com/grafana/gomemcache v0.0.0-20250318131618-74242eea118d
go: downloading github.com/grafana/otel-profiling-go v0.5.1
go: downloading github.com/stretchr/objx v0.5.2
go: downloading github.com/opentracing/opentracing-go v1.2.1-0.20220228012449-10b1cf09e00b
go: downloading github.com/felixge/httpsnoop v1.0.4
go: downloading go4.org/netipx v0.0.0-20230125063823-8449b0a6169f
go: downloading github.com/opentracing-contrib/go-stdlib v1.1.0
go: downloading github.com/redis/go-redis/v9 v9.11.0
go: downloading github.com/grafana/pyroscope-go/godeltaprof v0.1.8
go: downloading go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0
go: downloading github.com/sony/gobreaker/v2 v2.1.0
go: downloading github.com/opentracing-contrib/go-grpc v0.1.2
go: downloading github.com/uber/jaeger-client-go v2.30.0+incompatible
go: downloading github.com/pires/go-proxyproto v0.7.0
go: downloading github.com/uber/jaeger-lib v2.4.1+incompatible
go: downloading github.com/sony/gobreaker v0.5.0
go: downloading github.com/sercand/kuberesolver/v6 v6.0.0
go: downloading github.com/miekg/dns v1.1.68
go: downloading go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0
go: downloading go.opentelemetry.io/contrib/exporters/autoexport v0.61.0
go: downloading github.com/prometheus/exporter-toolkit v0.14.0
go: downloading github.com/hashicorp/go-metrics v0.5.4
go: downloading go.opentelemetry.io/contrib/propagators/jaeger v1.35.0
go: downloading go.opentelemetry.io/contrib/samplers/jaegerremote v0.30.0
go: downloading go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.61.0
go: downloading go.opentelemetry.io/otel/exporters/jaeger v1.17.0
go: downloading github.com/hashicorp/go-sockaddr v1.0.7
go: downloading github.com/grafana/memberlist v0.3.1-0.20220714140823-09ffed8adbbe
go: downloading github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb
go: downloading github.com/hashicorp/consul/api v1.32.1
go: downloading go.etcd.io/etcd/api/v3 v3.5.16
go: downloading github.com/hashicorp/go-cleanhttp v0.5.2
go: downloading go.etcd.io/etcd/client/v3 v3.5.16
go: downloading go.etcd.io/etcd/client/pkg/v3 v3.5.16
go: downloading github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
go: downloading dario.cat/mergo v1.0.2
go: downloading github.com/Masterminds/goutils v1.1.1
go: downloading github.com/Masterminds/semver/v3 v3.4.0
go: downloading github.com/huandu/xstrings v1.5.0
go: downloading github.com/shopspring/decimal v1.4.0
go: downloading github.com/spf13/cast v1.9.2
go: downloading golang.org/x/crypto v0.40.0
go: downloading github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f
go: downloading github.com/go-redsync/redsync/v4 v4.13.0
go: downloading go.opentelemetry.io/contrib/bridges/prometheus v0.61.0
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.12.2
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.12.2
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.36.0
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.37.0
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.36.0
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0
go: downloading go.opentelemetry.io/otel/exporters/prometheus v0.58.0
go: downloading go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.12.2
go: downloading go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.36.0
go: downloading go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.36.0
go: downloading go.opentelemetry.io/otel/sdk/log v0.12.2
go: downloading go.opentelemetry.io/otel/sdk/metric v1.37.0
go: downloading github.com/fsnotify/fsnotify v1.9.0
go: downloading github.com/jaegertracing/jaeger-idl v0.5.0
go: downloading github.com/hashicorp/go-immutable-radix v1.3.1
go: downloading github.com/coreos/go-systemd/v22 v22.5.0
go: downloading github.com/mdlayher/vsock v1.2.1
go: downloading golang.org/x/sync v0.16.0
go: downloading github.com/armon/go-metrics v0.4.1
go: downloading github.com/hashicorp/go-msgpack v1.1.5
go: downloading github.com/google/btree v1.1.3
go: downloading github.com/hashicorp/go-multierror v1.1.1
go: downloading github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529
go: downloading github.com/jpillora/backoff v1.0.0
go: downloading google.golang.org/genproto/googleapis/api v0.0.0-20250728155136-f173205681a0
go: downloading github.com/hashicorp/go-hclog v1.6.3
go: downloading github.com/hashicorp/go-rootcerts v1.0.2
go: downloading github.com/hashicorp/serf v0.10.2
go: downloading github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c
go: downloading golang.org/x/exp v0.0.0-20250620022241-b7579e27df2b
go: downloading go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.37.0
go: downloading github.com/klauspost/compress v1.18.0
go: downloading github.com/dennwc/varint v1.0.0
go: downloading github.com/edsrzf/mmap-go v1.2.0
go: downloading github.com/coreos/go-semver v0.3.1
go: downloading github.com/coreos/go-systemd v0.0.0-20191104093116-d3cd4ed1dbcf
go: downloading github.com/hashicorp/errwrap v1.1.0
go: downloading go.opentelemetry.io/proto/otlp v1.7.1
go: downloading github.com/cenkalti/backoff/v5 v5.0.2
go: downloading github.com/cenkalti/backoff v2.2.1+incompatible
go: downloading github.com/hashicorp/golang-lru v1.0.2
go: downloading github.com/mdlayher/socket v0.5.1
go: downloading github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.1

________________________________________________________
Executed in   21.21 secs    fish           external
   usr time   10.39 secs    0.00 micros   10.39 secs
   sys time    4.03 secs  777.00 micros    4.03 secs

total 78M
-rw-r--r--. 1 korniltsev korniltsev 163 Sep  6 13:05 hello_test.go
-rwxr-xr-x. 1 korniltsev korniltsev 78M Sep  6 13:07 test.test*
```

TLDR: 21 seconds to download 216 dependencies and 78M resulting binary.

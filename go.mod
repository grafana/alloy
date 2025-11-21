module github.com/grafana/alloy

go 1.25.1

require (
	cloud.google.com/go/pubsub v1.50.1
	cloud.google.com/go/pubsub/v2 v2.0.0
	connectrpc.com/connect v1.18.1
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.1
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.12.0
	github.com/Azure/go-autorest/autorest v0.11.30
	github.com/BurntSushi/toml v1.5.0
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/DataDog/go-sqllexer v0.1.9
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.54.0
	github.com/IBM/sarama v1.46.3
	github.com/KimMachineGun/automemlimit v0.7.4
	github.com/Lusitaniae/apache_exporter v0.11.1-0.20220518131644-f9522724dab4
	github.com/Masterminds/goutils v1.1.1
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/PuerkitoBio/rehttp v1.4.0
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b
	github.com/aws/aws-sdk-go-v2 v1.39.4
	github.com/aws/aws-sdk-go-v2/config v1.31.15
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.11
	github.com/aws/aws-sdk-go-v2/service/s3 v1.89.0
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.39.12
	github.com/blang/semver/v4 v4.0.0
	github.com/bmatcuk/doublestar v1.3.4
	github.com/boynux/squid-exporter v1.10.5-0.20230618153315-c1fae094e18e
	github.com/buger/jsonparser v1.1.1
	github.com/burningalchemist/sql_exporter v0.0.0-20240103092044-466b38b6abc4
	github.com/c2h5oh/datasize v0.0.0-20231215233829-aa82cc1e6500
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/coreos/go-systemd/v22 v22.6.0
	github.com/dimchansky/utfbom v1.1.1
	github.com/docker/docker v28.5.1+incompatible
	github.com/docker/go-connections v0.6.0
	github.com/drone/envsubst/v2 v2.0.0-20210730161058-179042472c46
	github.com/elastic/go-freelru v0.16.0 // indirect
	github.com/fatih/color v1.18.0
	github.com/fortytw2/leaktest v1.3.0
	github.com/fsnotify/fsnotify v1.9.0
	github.com/github/smimesign v0.2.0
	github.com/githubexporter/github-exporter v0.0.0-20231025122338-656e7dc33fe7
	github.com/go-git/go-git/v5 v5.16.2
	github.com/go-kit/kit v0.13.0
	github.com/go-kit/log v0.2.1
	github.com/go-logfmt/logfmt v0.6.1
	github.com/go-logr/logr v1.4.3
	github.com/go-sourcemap/sourcemap v2.1.4+incompatible
	github.com/go-sql-driver/mysql v1.9.3
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.4
	github.com/golang/snappy v1.0.0
	github.com/google/cadvisor v0.47.0
	github.com/google/dnsmasq_exporter v0.2.1-0.20230620100026-44b14480804a
	github.com/google/go-cmp v0.7.0
	github.com/google/pprof v0.0.0-20250923004556-9e5a51aed1e8
	github.com/google/renameio/v2 v2.0.0
	github.com/google/uuid v1.6.0
	github.com/gorilla/mux v1.8.1
	github.com/grafana/alloy-remote-config v0.0.12
	github.com/grafana/alloy/syntax v0.1.0
	github.com/grafana/beyla/v2 v2.7.8
	github.com/grafana/catchpoint-prometheus-exporter v0.0.0-20250218151502-6e97feaee761
	github.com/grafana/ckit v0.0.0-20251024151910-87043f5a3cf7
	github.com/grafana/cloudflare-go v0.0.0-20230110200409-c627cf6792f2
	github.com/grafana/dskit v0.0.0-20250828173137-de14cf923eeb
	github.com/grafana/go-gelf/v2 v2.0.1
	github.com/grafana/jfr-parser/pprof v0.0.4
	github.com/grafana/jsonparser v0.0.0-20241004153430-023329977675
	github.com/grafana/kafka_exporter v0.0.0-20240409084445-5e3488ad9f9a
	github.com/grafana/loki/pkg/push v0.0.0-20251021174646-053429db2124
	github.com/grafana/loki/v3 v3.0.0-20251021174646-053429db2124 // main branch (commit 053429db212405966051e8410ccf786db26065f1) - replace once there is a release that upgrades Loki's Prometheus.
	github.com/grafana/pyroscope-go/godeltaprof v0.1.8
	github.com/grafana/pyroscope/api v1.2.0
	github.com/grafana/pyroscope/ebpf v0.4.11
	github.com/grafana/pyroscope/lidia v0.0.0-20250716102313-506840f4afcd
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853
	github.com/grafana/snowflake-prometheus-exporter v0.0.0-20251023151319-9baba332b98a
	github.com/grafana/vmware_exporter v0.0.5-beta.0.20250218170317-73398ba08329
	github.com/grafana/walqueue v0.0.0-20251021201828-bb0dba7569b6
	github.com/hashicorp/consul/api v1.32.1
	github.com/hashicorp/go-discover v1.1.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v1.0.2
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/hashicorp/vault/api v1.20.0
	github.com/hashicorp/vault/api/auth/approle v0.2.0
	github.com/hashicorp/vault/api/auth/aws v0.2.0
	github.com/hashicorp/vault/api/auth/azure v0.2.0
	github.com/hashicorp/vault/api/auth/gcp v0.9.0
	github.com/hashicorp/vault/api/auth/kubernetes v0.2.0
	github.com/hashicorp/vault/api/auth/ldap v0.2.0
	github.com/hashicorp/vault/api/auth/userpass v0.9.0
	github.com/heroku/x v0.5.2
	github.com/influxdata/influxdb-client-go/v2 v2.14.0
	github.com/influxdata/influxdb1-client v0.0.0-20220302092344-a9ab5670611c
	github.com/influxdata/telegraf v1.35.2
	github.com/jaegertracing/jaeger-idl v0.6.0
	github.com/jaswdr/faker/v2 v2.8.0
	github.com/jmespath-community/go-jmespath v1.1.1
	github.com/jonboulle/clockwork v0.5.0
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.18.1
	github.com/leodido/go-syslog/v4 v4.3.0
	github.com/lib/pq v1.10.9
	github.com/mackerelio/go-osstat v0.2.5
	github.com/miekg/dns v1.1.68
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f
	github.com/natefinch/atomic v1.0.1
	github.com/ncabatoff/process-exporter v0.8.7
	github.com/oklog/run v1.2.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/oliver006/redis_exporter v1.74.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.138.0 // bug with v139 https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/43950
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.130.0 // TODO: Upgrade this when we can use a new version of otlptranslator
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki v0.130.0 // TODO: Upgrade this when we can use a new version of otlptranslator
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.133.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.139.0
	github.com/oracle/oracle-db-appdev-monitoring v0.0.0-20250516154730-1d8025fde3b0
	github.com/ory/dockertest/v3 v3.12.0
	github.com/oschwald/geoip2-golang v1.13.0
	github.com/oschwald/maxminddb-golang v1.13.1
	github.com/percona/mongodb_exporter v0.45.1-0.20250630080259-d761c954bba6
	github.com/phayes/freeport v0.0.0-20220201140144-74d24b5ae9f5
	github.com/pingcap/tidb/pkg/parser v0.0.0-20250501143621-a50a2323f4ba
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/prometheus-community/elasticsearch_exporter v1.5.0
	github.com/prometheus-community/postgres_exporter v0.18.1
	github.com/prometheus-community/stackdriver_exporter v0.18.0
	github.com/prometheus-community/windows_exporter v0.31.3 // if you update the windows_exporter version, make sure to update the PROM_WIN_EXP_VERSION in _index
	github.com/prometheus-community/yet-another-cloudwatch-exporter v0.62.1
	github.com/prometheus-operator/prometheus-operator v0.86.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.86.1
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.86.1
	github.com/prometheus/alertmanager v0.28.1
	github.com/prometheus/blackbox_exporter v0.24.1-0.20230623125439-bd22efa1c900
	github.com/prometheus/client_golang v1.23.2
	github.com/prometheus/client_model v0.6.2
	github.com/prometheus/common v0.67.1
	github.com/prometheus/consul_exporter v0.8.0
	github.com/prometheus/memcached_exporter v0.13.0
	github.com/prometheus/mysqld_exporter v0.17.2
	github.com/prometheus/node_exporter v1.9.1
	github.com/prometheus/procfs v0.17.0
	github.com/prometheus/prometheus v0.305.1-0.20250818080900-0a40df33fb4e // replaced by a fork of v3.7.1 further down this file
	github.com/prometheus/sigv4 v0.2.1
	github.com/prometheus/snmp_exporter v0.29.0 // if you update the snmp_exporter version, make sure to update the SNMP_VERSION in _index
	github.com/prometheus/statsd_exporter v0.28.0
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3
	github.com/rogpeppe/go-internal v1.14.1
	github.com/rs/cors v1.11.1
	github.com/samber/lo v1.51.0
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.35
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/sijms/go-ora/v2 v2.9.0
	github.com/sirupsen/logrus v1.9.3
	github.com/spaolacci/murmur3 v1.1.0
	github.com/spf13/cobra v1.10.1
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.39.0
	github.com/tilinna/clock v1.1.0
	github.com/uber/jaeger-client-go v2.30.0+incompatible
	github.com/vincent-petithory/dataurl v1.0.0
	github.com/webdevops/azure-metrics-exporter v0.0.0-20230717202958-8701afc2b013
	github.com/webdevops/go-common v0.0.0-20250617214056-2620f947754f
	github.com/wk8/go-ordered-map v0.2.0
	github.com/xdg-go/scram v1.1.2
	github.com/xwb1989/sqlparser v0.0.0-20180606152119-120387863bf2 // indirect
	github.com/zeebo/xxh3 v1.0.2
	go.opentelemetry.io/collector/client v1.45.0
	go.opentelemetry.io/collector/component v1.45.0
	go.opentelemetry.io/collector/component/componentstatus v0.139.0
	go.opentelemetry.io/collector/component/componenttest v0.139.0
	go.opentelemetry.io/collector/config/configauth v1.45.0
	go.opentelemetry.io/collector/config/configcompression v1.45.0
	go.opentelemetry.io/collector/config/configgrpc v0.139.0
	go.opentelemetry.io/collector/config/confighttp v0.139.0
	go.opentelemetry.io/collector/config/confignet v1.45.0
	go.opentelemetry.io/collector/config/configopaque v1.45.0
	go.opentelemetry.io/collector/config/configoptional v1.45.0
	go.opentelemetry.io/collector/config/configretry v1.45.0
	go.opentelemetry.io/collector/config/configtelemetry v0.139.0
	go.opentelemetry.io/collector/config/configtls v1.45.0
	go.opentelemetry.io/collector/confmap v1.45.0
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.45.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.139.0
	go.opentelemetry.io/collector/connector v0.139.0
	go.opentelemetry.io/collector/connector/connectortest v0.139.0
	go.opentelemetry.io/collector/consumer v1.45.0
	go.opentelemetry.io/collector/consumer/consumertest v0.139.0
	go.opentelemetry.io/collector/exporter v1.45.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.139.0
	go.opentelemetry.io/collector/exporter/exporterhelper v0.139.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.139.0
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.139.0
	go.opentelemetry.io/collector/extension v1.45.0
	go.opentelemetry.io/collector/extension/extensionauth v1.45.0
	go.opentelemetry.io/collector/extension/extensiontest v0.139.0
	go.opentelemetry.io/collector/extension/xextension v0.139.0
	go.opentelemetry.io/collector/featuregate v1.45.0
	go.opentelemetry.io/collector/otelcol v0.139.0
	go.opentelemetry.io/collector/pdata v1.45.0
	go.opentelemetry.io/collector/pipeline v1.45.0
	go.opentelemetry.io/collector/processor v1.45.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.139.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.139.0
	go.opentelemetry.io/collector/receiver v1.45.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.139.0
	go.opentelemetry.io/collector/receiver/receiverhelper v0.139.0
	go.opentelemetry.io/collector/receiver/receivertest v0.139.0
	go.opentelemetry.io/collector/scraper/scraperhelper v0.139.0
	go.opentelemetry.io/collector/service v0.139.0
	go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux v0.45.0
	go.opentelemetry.io/ebpf-profiler v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/obi v1.2.2
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.38.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.38.0
	go.opentelemetry.io/otel/exporters/prometheus v0.60.0
	go.opentelemetry.io/otel/metric v1.38.0
	go.opentelemetry.io/otel/sdk v1.38.0
	go.opentelemetry.io/otel/sdk/metric v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	go.opentelemetry.io/proto/otlp v1.8.0
	go.uber.org/atomic v1.11.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.45.0
	golang.org/x/crypto/x509roots/fallback v0.0.0-20240208163226-62c9f1799c91
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546
	golang.org/x/net v0.47.0
	golang.org/x/oauth2 v0.32.0
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.38.0
	golang.org/x/text v0.31.0
	golang.org/x/time v0.14.0
	golang.org/x/tools v0.38.0
	google.golang.org/api v0.254.0
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	gotest.tools v2.2.0+incompatible
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
	k8s.io/component-base v0.34.1
	k8s.io/klog/v2 v2.130.1
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4
	sigs.k8s.io/controller-runtime v0.22.2
	sigs.k8s.io/yaml v1.6.0
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	cloud.google.com/go/logging v1.13.0 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/monitoring v1.24.2 // indirect
	cloud.google.com/go/trace v1.11.6 // indirect
	dario.cat/mergo v1.0.2 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.2 // indirect
	github.com/AlekSi/pointer v1.2.0 // indirect
	github.com/AlessandroPomponio/go-gibberish v0.0.0-20191004143433-a2d4156f0396 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets v0.12.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/keyvault/internal v0.7.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor v0.11.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph v0.9.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources v1.2.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.6.1 // indirect
	github.com/Azure/go-amqp v1.5.0 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.24 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.13 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.5.0 // indirect
	github.com/ClickHouse/clickhouse-go v1.5.4 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.171 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/logscompression v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/client v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/diagnostic v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/message v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/metrics v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/pipeline v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/processor v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sender v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sources v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/utils v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/metrics v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/tagset v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/common v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/otel v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/startstop v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/statstracker v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.48.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.8.1 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.14.1-0.20251008075154-b33ffa9792d9 // indirect
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/GehirnInc/crypt v0.0.0-20230320061759-8cc1b52080c5 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.54.0 // indirect
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/Shopify/sarama v1.38.1 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/Workiva/go-datastructures v1.1.6 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/andybalholm/brotli v1.2.0 // indirect
	github.com/antchfx/xmlquery v1.5.0 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/apache/arrow-go/v18 v18.4.0 // indirect
	github.com/apache/thrift v0.22.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-msk-iam-sasl-signer-go v1.0.4 // indirect
	github.com/aws/aws-sdk-go v1.55.7 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.2 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.19 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/amp v1.30.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/apigateway v1.28.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/apigatewayv2 v1.24.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/autoscaling v1.51.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.45.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.58.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/databasemigrationservice v1.45.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.259.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecs v1.53.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/iam v1.38.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/lightsail v1.49.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi v1.25.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.27.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/shield v1.29.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/storagegateway v1.34.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.9 // indirect
	github.com/aws/smithy-go v1.23.1 // indirect
	github.com/axiomhq/hyperloglog v0.2.5 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bboreham/go-loser v0.0.0-20230920113527-fcc2c21820a3 // indirect
	github.com/beevik/ntp v1.3.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.22.0 // indirect
	github.com/bits-and-blooms/bloom/v3 v3.7.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.9.1 // indirect
	github.com/caarlos0/env/v9 v9.0.0
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/channelmeter/iso8601duration v0.0.0-20150204201828-8da3af7a2a61
	github.com/checkpoint-restore/go-criu/v6 v6.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/cilium/ebpf v0.19.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cloudflare/golz4 v0.0.0-20150217214814-ef862a3cdc58 // indirect
	github.com/cncf/xds/go v0.0.0-20250501225837-2ac532fd4443 // indirect
	github.com/containerd/console v1.0.5 // indirect
	github.com/containerd/continuity v0.4.5 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v1.0.0-rc.1 // indirect
	github.com/containerd/ttrpc v1.2.7 // indirect
	github.com/containers/common v0.64.2 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/creack/pty v1.1.24 // indirect
	github.com/cyphar/filepath-securejoin v0.6.0 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/deckarep/golang-set/v2 v2.6.0 // indirect
	github.com/deneonet/benc v1.1.7 // indirect
	github.com/dennwc/btrfs v0.0.0-20230312211831-a1f570bd01a1 // indirect
	github.com/dennwc/ioctl v1.0.0 // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/denverdino/aliyungo v0.0.0-20190125010748-a747050bb1ba // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/digitalocean/godo v1.165.1 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/cli v28.1.1+incompatible // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/dvsekhvalnov/jose2go v1.6.0 // indirect
	github.com/eapache/go-resiliency v1.7.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20230731223053-c322873962e3 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/efficientgo/core v1.0.0-rc.3 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/go-perf v0.0.0-20241029065020-30bec95324b8 // indirect
	github.com/elastic/go-sysinfo v1.8.1 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/ema/qdisc v1.0.0 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.35.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/expr-lang/expr v1.17.6 // indirect
	github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/gavv/monotime v0.0.0-20190418164738-30dba4353424 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/go-jose/go-jose/v4 v4.1.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.24.0 // indirect
	github.com/go-openapi/errors v0.22.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.2 // indirect
	github.com/go-openapi/loads v0.23.1 // indirect
	github.com/go-openapi/runtime v0.29.0 // indirect
	github.com/go-openapi/spec v0.22.0 // indirect
	github.com/go-openapi/strfmt v0.24.0 // indirect
	github.com/go-openapi/swag v0.25.1 // indirect
	github.com/go-openapi/validate v0.25.0 // indirect
	github.com/go-resty/resty/v2 v2.16.5 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/godror/godror v0.48.1 // indirect
	github.com/godror/knownpb v0.1.2 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/status v1.1.1 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/gomodule/redigo v1.9.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/go-tpm v0.9.6 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gophercloud/gophercloud v1.14.1 // indirect
	github.com/gophercloud/gophercloud/v2 v2.8.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosnmp/gosnmp v1.41.0 // indirect
	github.com/grafana/faro/pkg/go v0.0.0-20250314155512-06a06da3b8bc // indirect
	github.com/grafana/go-offsets-tracker v0.1.7 // indirect
	github.com/grafana/gomemcache v0.0.0-20250828162811-a96f6acee2fe // indirect
	github.com/grafana/jfr-parser v0.9.3 // indirect
	github.com/grafana/jvmtools v0.0.3 // indirect
	github.com/grafana/otel-profiling-go v0.5.1 // indirect
	github.com/grobie/gomemcache v0.0.0-20230213081705-239240bbc445 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.2 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/hashicorp/cronexpr v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-discover/provider/gce v0.0.0-20250625044749-2961509ab243 // indirect
	github.com/hashicorp/go-envparse v0.1.0 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-metrics v0.5.4 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/awsutil v0.3.0 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.6 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hashicorp/mdns v1.0.5 // indirect
	github.com/hashicorp/memberlist v0.5.3 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20250930071859-eaa0fe0e27af // indirect
	github.com/hashicorp/serf v0.10.2 // indirect
	github.com/hashicorp/vic v1.5.1-0.20190403131502-bbfe86ec9443 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.28.0 // indirect
	github.com/hodgesds/perf-utils v0.7.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/ianlancetaylor/demangle v0.0.0-20250628045327-2d64ad6b7ec5
	github.com/illumos/go-kstat v0.0.0-20210513183136-173c9b0a9973 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/infinityworks/go-common v0.0.0-20170820165359-7f20a140fd37 // indirect
	github.com/influxdata/influxdb-observability/common v0.5.12 // indirect
	github.com/influxdata/influxdb-observability/influx2otel v0.5.12 // indirect
	github.com/influxdata/line-protocol v0.0.0-20200327222509-2487e7298839 // indirect
	github.com/influxdata/line-protocol/v2 v2.2.1 // indirect
	github.com/influxdata/tdigest v0.0.2-0.20210216194612-fc98d27c9e8b // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.3.4 // indirect
	github.com/itchyny/timefmt-go v0.1.7 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.14.3 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.3 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgtype v1.14.4 // indirect
	github.com/jackc/pgx/v4 v4.18.3 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jessevdk/go-flags v1.6.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/joeshaw/multierror v0.0.0-20140124173710-69b34d4ec901 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/joyent/triton-go v1.8.5 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/jsimonetti/rtnetlink v1.4.2 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kamstrup/intmap v0.5.1 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.11 // indirect
	github.com/klauspost/pgzip v1.2.6 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/krallistic/kazoo-go v0.0.0-20170526135507-a15279744f4e // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/linode/linodego v1.59.0 // indirect
	github.com/lufia/iostat v1.2.1 // indirect
	github.com/lufia/plan9stats v0.0.0-20250827001030-24949be3fa54 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.9.1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.16 // indirect
	github.com/mattn/go-xmlrpc v0.0.3 // indirect
	github.com/mdlayher/ethtool v0.1.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/kobject v0.0.0-20200520190114-19ca17470d7d // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/mdlayher/wifi v0.1.0 // indirect
	github.com/metalmatze/signal v0.0.0-20210307161603-1c9aa721a97a // indirect
	github.com/microsoft/go-mssqldb v1.9.2 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mna/redisc v1.4.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.1.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mongodb/mongo-tools v0.0.0-20250715155915-887641268977 // indirect
	github.com/montanaflynn/stats v0.7.1 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/mrunalp/fileutils v0.5.1 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/ncabatoff/go-seq v0.0.0-20180805175032-b08ef85ed833 // indirect
	github.com/nicolai86/scaleway-sdk v1.10.2-0.20180628010248-798f60e20bb2 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/ohler55/ojg v1.26.8 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.139.0 // indirect; indirect)
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.139.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.139.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runc v1.3.3 // indirect
	github.com/opencontainers/runtime-spec v1.2.1 // indirect
	github.com/opencontainers/selinux v1.13.0 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/opentracing-contrib/go-grpc v0.1.2 // indirect
	github.com/opentracing-contrib/go-stdlib v1.1.0 // indirect
	github.com/opentracing/opentracing-go v1.2.1-0.20220228012449-10b1cf09e00b // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/oracle/oci-go-sdk/v65 v65.89.3 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/ovh/go-ovh v1.9.0 // indirect
	github.com/packethost/packngo v0.1.1-0.20180711074735-b9cb5096f54c // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/percona/percona-backup-mongodb v1.8.1-0.20250218045950-7e9f38fe06ab // indirect
	github.com/peterbourgon/ff/v3 v3.4.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pingcap/errors v0.11.5-0.20240311024730-e056997136bb // indirect
	github.com/pingcap/failpoint v0.0.0-20240528011301-b51a646c7c86 // indirect
	github.com/pingcap/log v1.1.0 // indirect
	github.com/pires/go-proxyproto v0.7.0 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus-community/go-runit v0.1.0 // indirect
	github.com/prometheus-community/prom-label-proxy v0.12.1 // indirect
	github.com/prometheus/exporter-toolkit v0.14.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20250401214520-65e299d6c5c9 // indirect
	github.com/redis/go-redis/v9 v9.11.0 // indirect
	github.com/relvacode/iso8601 v1.7.0 // indirect
	github.com/remeh/sizedwaitgroup v1.0.0 // indirect
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/safchain/ethtool v0.5.10 // indirect
	github.com/samuel/go-zookeeper v0.0.0-20190923202752-2cc03de413da // indirect
	github.com/sean-/seed v0.0.0-20170313163322-e2103e2c3529 // indirect
	github.com/seccomp/libseccomp-golang v0.11.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/sercand/kuberesolver/v6 v6.0.0 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/shirou/gopsutil/v4 v4.25.9 // indirect
	github.com/shoenig/go-m1cpu v0.1.7 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/snowflakedb/gosnowflake v1.14.1 // indirect
	github.com/softlayer/softlayer-go v0.0.0-20180806151055-260589d94c7d // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/sony/gobreaker v0.5.0 // indirect
	github.com/sony/gobreaker/v2 v2.3.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/stackitcloud/stackit-sdk-go/core v0.17.3 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.480 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm v1.0.480 // indirect
	github.com/tg123/go-htpasswd v1.2.4 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.2.1 // indirect
	github.com/tinylib/msgp v1.5.0 // indirect
	github.com/tjhop/slog-gokit v0.1.4 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80 // indirect
	github.com/twmb/franz-go v1.20.2 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.12.0 // indirect
	github.com/twmb/franz-go/pkg/sasl/kerberos v1.1.0 // indirect
	github.com/twmb/franz-go/plugin/kzap v1.1.2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vertica/vertica-sql-go v1.3.3 // indirect
	github.com/vishvananda/netlink v1.3.1 // indirect
	github.com/vishvananda/netns v0.0.5 // indirect
	github.com/vmware/govmomi v0.51.0 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/xdg/scram v1.0.5 // indirect
	github.com/xdg/stringprep v1.0.3 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xhit/go-str2duration/v2 v2.1.0 // indirect
	github.com/xo/dburl v0.20.0 // indirect
	github.com/yl2chen/cidranger v1.0.2 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.etcd.io/bbolt v1.4.3 // indirect
	go.etcd.io/etcd/api/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.6.4 // indirect
	go.etcd.io/etcd/client/v3 v3.6.4 // indirect
	go.mongodb.org/mongo-driver v1.17.4 // indirect
	go.mongodb.org/mongo-driver/v2 v2.3.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.139.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.45.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.139.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.139.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.139.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.139.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.139.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.139.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.139.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.139.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.139.0 // indirect
	go.opentelemetry.io/collector/filter v0.139.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.139.0 // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.139.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.139.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.139.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.139.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.139.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.139.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.139.0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper v0.139.0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.139.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.139.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.139.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.139.0 // indirect
	go.opentelemetry.io/collector/scraper v0.139.0 // indirect
	go.opentelemetry.io/collector/semconv v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.139.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.13.0 // indirect
	go.opentelemetry.io/contrib/bridges/prometheus v0.61.0 // indirect
	go.opentelemetry.io/contrib/detectors/aws/ec2/v2 v2.0.0 // indirect
	go.opentelemetry.io/contrib/detectors/aws/eks v1.38.0 // indirect
	go.opentelemetry.io/contrib/detectors/azure/azurevm v0.10.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.38.0 // indirect
	go.opentelemetry.io/contrib/exporters/autoexport v0.61.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0
	go.opentelemetry.io/contrib/otelconf v0.18.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.38.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.35.0
	go.opentelemetry.io/contrib/samplers/jaegerremote v0.30.0 // indirect
	go.opentelemetry.io/otel/exporters/jaeger v1.17.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.38.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.38.0 // indirect
	go.opentelemetry.io/otel/log v0.14.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.14.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go4.org/netipx v0.0.0-20230125063823-8449b0a6169f // indirect
	golang.design/x/chann v0.1.2 // indirect
	golang.org/x/arch v0.22.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto v0.0.0-20250922171735-9219d122eba9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250929231259-57b25ae835d4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	howett.net/plist v1.0.0 // indirect
	k8s.io/apiextensions-apiserver v0.34.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
)

require (
	cyphar.com/go-pathrs v0.2.1 // indirect
	github.com/DataDog/datadog-agent/comp/api/api/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/noop-impl v0.0.0-20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/def v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/types v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/logs v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/rum v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/defaultpaths v0.69.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.73.0-devel.0.20251030121902-cd89eab046d6 // indirect
	github.com/digitalocean/go-metadata v0.0.0-20250129100319-e3650a3df44b // indirect
	github.com/go-openapi/swag/cmdutils v0.25.1 // indirect
	github.com/go-openapi/swag/conv v0.25.1 // indirect
	github.com/go-openapi/swag/fileutils v0.25.1 // indirect
	github.com/go-openapi/swag/jsonname v0.25.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.1 // indirect
	github.com/go-openapi/swag/loading v0.25.1 // indirect
	github.com/go-openapi/swag/mangling v0.25.1 // indirect
	github.com/go-openapi/swag/netutils v0.25.1 // indirect
	github.com/go-openapi/swag/stringutils v0.25.1 // indirect
	github.com/go-openapi/swag/typeutils v0.25.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.1 // indirect
	github.com/linode/go-metadata v0.2.2 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/twmb/franz-go/pkg/kadm v1.17.1 // indirect
	golang.org/x/telemetry v0.0.0-20251008203120-078029d740a8 // indirect
)

// NOTE: replace directives below must always be *temporary*.
//
// Adding a replace directive to change a module to a fork of a module will
// only be accepted when a PR upstream has been opened to accept the new
// change.
//
// Contributors are expected to work with upstream to make their changes
// acceptable, and remove the `replace` directive as soon as possible.
//
// If upstream is unresponsive, you should consider making a hard fork
// (i.e., creating a new Go module with the same source) or picking a different
// dependency.

// TODO: remove this replace directive once the upstream issue is fixed: https://github.com/prometheus/prometheus/issues/13842
replace go.opentelemetry.io/collector/featuregate => github.com/grafana/opentelemetry-collector/featuregate v0.0.0-20240325174506-2fd1623b2ca0 // feature-gate-registration-error-handler branch

// Replace directives from Prometheus
replace github.com/fsnotify/fsnotify v1.8.0 => github.com/fsnotify/fsnotify v1.7.0

// TODO: remove replace directive once:
// * There is a release of Prometheus which addresses https://github.com/prometheus/prometheus/issues/14049,
// for example, via this implementation: https://github.com/grafana/prometheus/pull/34
replace github.com/prometheus/prometheus => github.com/grafana/prometheus v1.8.2-0.20251030104821-c9e0b31e9aeb // staleness_disabling_v3.7.3 branch

replace gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc

// Replace directives from Loki
replace (
	github.com/Azure/azure-sdk-for-go => github.com/Azure/azure-sdk-for-go v68.0.0+incompatible
	github.com/Azure/azure-storage-blob-go => github.com/MasslessParticle/azure-storage-blob-go v0.14.1-0.20240322194317-344980fda573
	// Use fork of gocql that has gokit logs and Prometheus metrics.
	github.com/gocql/gocql => github.com/grafana/gocql v0.0.0-20200605141915-ba5dc39ece85
	// Insist on the optimised version of grafana/regexp
	github.com/grafana/regexp => github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc
	// Replace memberlist with our fork which includes some fixes that haven't been
	// merged upstream yet.
	github.com/hashicorp/memberlist => github.com/grafana/memberlist v0.3.1-0.20220714140823-09ffed8adbbe
	// leodido fork his project to continue support
	github.com/influxdata/go-syslog/v3 => github.com/leodido/go-syslog/v4 v4.2.0
	github.com/thanos-io/objstore => github.com/grafana/objstore v0.0.0-20250210100727-533688b5600d
)

// TODO(rfratto): remove forks when changes are merged upstream
replace (
	// TODO(tpaschalis) this is to remove global instantiation of plugins
	// and allow non-singleton components.
	// https://github.com/grafana/cadvisor/tree/grafana-v0.47-noglobals
	github.com/google/cadvisor => github.com/grafana/cadvisor v0.0.0-20240729082359-1f04a91701e2

	// TODO(dehaansa,cristiangreco): this tracks the exporter-package-v0.18.1 branch of the grafana fork,
	// which we could get rid of once the changes are merged upstream.
	github.com/prometheus-community/postgres_exporter => github.com/grafana/postgres_exporter v0.0.0-20250930111128-c8f6a9f4d363

	// TODO(marctc): remove once this PR is merged upstream: https://github.com/prometheus/mysqld_exporter/pull/774
	github.com/prometheus/mysqld_exporter => github.com/grafana/mysqld_exporter v0.17.2-0.20250226152553-be612e3fdedd

	// TODO(marctc, mattdurham): Replace node_export with custom fork for multi usage. https://github.com/prometheus/node_exporter/pull/2812
	// this commit is on the refactor_collectors branch in the grafana fork.
	github.com/prometheus/node_exporter => github.com/grafana/node_exporter v0.18.1-grafana-r01.0.20251024135609-318b01780c89 //refactor_collectors
)

replace github.com/github/smimesign => github.com/grafana/smimesign v0.2.1-0.20220408144937-2a5adf3481d3

// Submodules.
replace github.com/grafana/alloy/syntax => ./syntax

// Add exclude directives so Go doesn't pick old incompatible k8s.io/client-go
// versions.
exclude (
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/client-go v12.0.0+incompatible
)

// replace go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation v1.3.9
replace go.opentelemetry.io/obi => github.com/grafana/opentelemetry-ebpf-instrumentation v1.3.10

replace go.opentelemetry.io/ebpf-profiler => github.com/grafana/opentelemetry-ebpf-profiler v0.0.202546-0.20251106085643-a00a0ef2a84c

// Update openshift/client-go to a version compatible with structured-merge-diff v6
replace github.com/openshift/client-go => github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235

// Use Grafana's patched k8sattributesprocessor that supports k8s.io/client-go v0.34.1
// Adds RunWithContext and AddEventHandlerWithOptions methods to fake informers
replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => github.com/grafana/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.0.0-20251021125353-73458b01ab23

// Do not remove this until the bug breaking backwards compatibility is resolved & included in walqueue: https://github.com/deneonet/benc/issues/13
replace github.com/deneonet/benc => github.com/deneonet/benc v1.1.7

// Pin runc to v1.1.12 to maintain compatibility with cadvisor that requires libcontainer/cgroups packages
replace github.com/opencontainers/runc => github.com/opencontainers/runc v1.2.8

replace sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.20.4

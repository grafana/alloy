package configgen

import (
	"fmt"

	"net/url"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/grafana/alloy/internal/component/common/kubernetes"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promConfig "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func boolPtr(val bool) *bool {
	return &val
}

func stringPtr(val string) *string {
	return &val
}

var (
	configGen = &ConfigGenerator{
		Secrets: &fakeSecrets{},
		AdditionalRelabelConfigs: []*alloy_relabel.Config{
			{TargetLabel: "__meta_foo", Replacement: "bar"},
		},
	}
)

func TestGenerateK8SSDConfig(t *testing.T) {
	nsDiscovery := promk8s.NamespaceDiscovery{
		Names: []string{""},
	}

	tests := []struct {
		name           string
		client         *kubernetes.ClientArguments
		attachMetadata *promopv1.AttachMetadata
		expected       *promk8s.SDConfig
	}{
		{
			name: "empty",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
			},
		},
		{
			name: "kubeconfig",
			client: &kubernetes.ClientArguments{
				KubeConfig: "kubeconfig",
				APIServer:  config.URL{},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				KubeConfig:         "kubeconfig",
			},
		},
		{
			name: "attach metadata",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{},
			},
			attachMetadata: &promopv1.AttachMetadata{
				Node: boolPtr(true),
			},
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				AttachMetadata:     promk8s.AttachMetadataConfig{Node: true},
			},
		},
		{
			name: "http client config",
			client: &kubernetes.ClientArguments{
				APIServer: config.URL{
					URL: &url.URL{
						Scheme: "https",
						Host:   "localhost:8080",
					},
				},
				HTTPClientConfig: config.HTTPClientConfig{
					BasicAuth: &config.BasicAuth{
						Username: "username",
						Password: "password",
					},
					BearerToken:     "bearer",
					BearerTokenFile: "bearer_file",
					TLSConfig: config.TLSConfig{
						CAFile:   "ca_file",
						CertFile: "cert_file",
					},
					Authorization: &config.Authorization{
						Credentials: "credentials",
					},
				},
			},
			attachMetadata: nil,
			expected: &promk8s.SDConfig{
				Role:               promk8s.RoleEndpoint,
				NamespaceDiscovery: nsDiscovery,
				APIServer: promConfig.URL{
					URL: &url.URL{
						Scheme: "https",
						Host:   "localhost:8080",
					},
				},
				HTTPClientConfig: promConfig.HTTPClientConfig{
					BasicAuth: &promConfig.BasicAuth{
						Username: "username",
						Password: "password",
					},
					BearerToken:     "bearer",
					BearerTokenFile: "bearer_file",
					TLSConfig: promConfig.TLSConfig{
						CAFile:   "ca_file",
						CertFile: "cert_file",
					},
					Authorization: &promConfig.Authorization{
						Type:        "Bearer",
						Credentials: "credentials",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cg := &ConfigGenerator{
				Client: tt.client,
			}
			got := cg.generateK8SSDConfig(promopv1.NamespaceSelector{}, "", promk8s.RoleEndpoint, tt.attachMetadata)
			assert.Equal(t, tt.expected, got)
		})
	}
}

type fakeSecrets struct{}

func (f *fakeSecrets) GetSecretValue(namespace string, sec corev1.SecretKeySelector) (string, error) {
	return fmt.Sprintf("secret/%s/%s/%s", namespace, sec.Name, sec.Key), nil
}
func (f *fakeSecrets) GetConfigMapValue(namespace string, cm corev1.ConfigMapKeySelector) (string, error) {
	return fmt.Sprintf("cm/%s/%s/%s", namespace, cm.Name, cm.Key), nil
}
func (f *fakeSecrets) SecretOrConfigMapValue(namespace string, socm promopv1.SecretOrConfigMap) (string, error) {
	if socm.Secret != nil {
		return f.GetSecretValue(namespace, *socm.Secret)
	}
	return f.GetConfigMapValue(namespace, *socm.ConfigMap)
}

// convenience functions for generating references
func s(name, key string) *corev1.SecretKeySelector {
	return &corev1.SecretKeySelector{
		Key: key,
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
	}
}
func cm(name, key string) *corev1.ConfigMapKeySelector {
	return &corev1.ConfigMapKeySelector{
		Key: key,
		LocalObjectReference: corev1.LocalObjectReference{
			Name: name,
		},
	}
}
func TestGenerateSafeTLSConfig(t *testing.T) {
	tests := []struct {
		name       string
		tlsConfig  promopv1.SafeTLSConfig
		hasErr     bool
		serverName string
		ca         string
		cert       string
		key        promConfig.Secret
	}{
		{
			name: "empty",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				ServerName:         stringPtr("test"),
			},
			hasErr:     false,
			serverName: "test",
		},
		{
			name: "ca_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				CA:                 promopv1.SecretOrConfigMap{Secret: s("secrets", "ca_file")},
			},
			hasErr:     false,
			serverName: "",
			ca:         "secret/ns/secrets/ca_file",
		},
		{
			name: "ca_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				CA:                 promopv1.SecretOrConfigMap{ConfigMap: cm("non-secrets", "ca_file")},
			},
			hasErr:     false,
			serverName: "",
			ca:         "cm/ns/non-secrets/ca_file",
		},
		{
			name: "cert_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				Cert:               promopv1.SecretOrConfigMap{Secret: s("secrets", "cert_file")},
			},
			hasErr:     false,
			serverName: "",
			cert:       "secret/ns/secrets/cert_file",
		},
		{
			name: "cert_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				Cert:               promopv1.SecretOrConfigMap{ConfigMap: cm("non-secrets", "cert_file")},
			},
			hasErr:     false,
			serverName: "",
			cert:       "cm/ns/non-secrets/cert_file",
		},
		{
			name: "key_file",
			tlsConfig: promopv1.SafeTLSConfig{
				InsecureSkipVerify: boolPtr(true),
				KeySecret:          s("secrets", "key_file"),
			},
			hasErr:     false,
			serverName: "",
			key:        "secret/ns/secrets/key_file",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateSafeTLS(tt.tlsConfig, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.True(t, got.InsecureSkipVerify)
			assert.Equal(t, tt.serverName, got.ServerName)
			assert.Equal(t, tt.ca, got.CA)
			assert.Equal(t, tt.cert, got.Cert)
			assert.Equal(t, tt.key, got.Key)
		})
	}
}

func TestGenerateTLSConfig(t *testing.T) {
	tests := []struct {
		name      string
		tlsConfig promopv1.TLSConfig
		hasErr    bool
		caFile    string
		certFile  string
		keyFile   string
		insecure  bool
	}{
		{
			name:      "empty",
			tlsConfig: promopv1.TLSConfig{},
			hasErr:    false,
		},
		{
			name: "all_fields",
			tlsConfig: promopv1.TLSConfig{
				SafeTLSConfig: promopv1.SafeTLSConfig{},
				CAFile:        "ca_file",
				KeyFile:       "key_file",
				CertFile:      "cert_file",
			},
			hasErr:   false,
			keyFile:  "key_file",
			caFile:   "ca_file",
			certFile: "cert_file",
		},
		{
			name: "safe gets set",
			tlsConfig: promopv1.TLSConfig{
				SafeTLSConfig: promopv1.SafeTLSConfig{
					InsecureSkipVerify: boolPtr(true),
				},
			},
			hasErr:   false,
			insecure: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateTLSConfig(tt.tlsConfig, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.Equal(t, tt.insecure, got.InsecureSkipVerify)
			assert.Equal(t, tt.caFile, got.CAFile)
			assert.Equal(t, tt.certFile, got.CertFile)
			assert.Equal(t, tt.keyFile, got.KeyFile)
		})
	}
}

func TestGenerateBasicAuth(t *testing.T) {
	un := s("s", "un")
	pw := s("s", "pw")
	tests := []struct {
		name     string
		ba       promopv1.BasicAuth
		hasErr   bool
		username string
		password promConfig.Secret
	}{
		{
			name: "empty",
			ba: promopv1.BasicAuth{
				Username: *un,
				Password: *pw,
			},
			hasErr:   false,
			username: "secret/ns/s/un",
			password: "secret/ns/s/pw",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateBasicAuth(tt.ba, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.Equal(t, tt.password, got.Password)
			assert.Equal(t, tt.username, got.Username)
		})
	}
}

func TestGenerateOauth2(t *testing.T) {
	cidSecret := promopv1.SecretOrConfigMap{Secret: s("oa", "cid")}
	cidCMap := promopv1.SecretOrConfigMap{ConfigMap: cm("oa", "cid")}
	cSecret := s("oa", "csecret")
	tests := []struct {
		name   string
		oa2    promopv1.OAuth2
		hasErr bool
		id     string
		secret promConfig.Secret
	}{
		{
			name: "id from secret",
			oa2: promopv1.OAuth2{
				ClientID:     cidSecret,
				ClientSecret: *cSecret,
			},
			hasErr: false,
			id:     "secret/ns/oa/cid",
			secret: "secret/ns/oa/csecret",
		},
		{
			name: "id from config map",
			oa2: promopv1.OAuth2{
				ClientID:     cidCMap,
				ClientSecret: *cSecret,
			},
			hasErr: false,
			id:     "cm/ns/oa/cid",
			secret: "secret/ns/oa/csecret",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateOauth2(tt.oa2, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.Equal(t, tt.id, got.ClientID)
			assert.Equal(t, tt.secret, got.ClientSecret)
		})
	}
}

func TestGenerateAuthorization(t *testing.T) {
	pw := s("s", "pw")
	tests := []struct {
		name   string
		auth   promopv1.SafeAuthorization
		hasErr bool
		creds  promConfig.Secret
		Type   string
	}{
		{
			name: "empty",
			auth: promopv1.SafeAuthorization{
				Credentials: pw,
			},
			hasErr: false,
			creds:  "secret/ns/s/pw",
			Type:   "Bearer",
		},
		{
			name: "type provided",
			auth: promopv1.SafeAuthorization{
				Credentials: pw,
				Type:        "Foo",
			},
			hasErr: false,
			creds:  "secret/ns/s/pw",
			Type:   "Foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configGen.generateAuthorization(tt.auth, "ns")
			assert.Equal(t, tt.hasErr, err != nil)
			assert.Equal(t, tt.creds, got.Credentials)
			assert.Equal(t, tt.Type, got.Type)
		})
	}
}

func TestGenerateDefaultScrapeConfig(t *testing.T) {
	tests := []struct {
		name                     string
		scrapeOptions            operator.ScrapeOptions
		expectedInterval         time.Duration
		expectedTimeout          time.Duration
		expectedFallbackProtocol promconfig.ScrapeProtocol
	}{
		{
			name:                     "empty",
			scrapeOptions:            operator.ScrapeOptions{},
			expectedInterval:         1 * time.Minute,
			expectedTimeout:          10 * time.Second,
			expectedFallbackProtocol: promconfig.PrometheusText0_0_4,
		},
		{
			name: "defaults set",
			scrapeOptions: operator.ScrapeOptions{
				DefaultScrapeInterval: 30 * time.Second,
				DefaultScrapeTimeout:  5 * time.Second,
				DefaultSampleLimit:    100,
			},
			expectedInterval:         30 * time.Second,
			expectedTimeout:          5 * time.Second,
			expectedFallbackProtocol: promconfig.PrometheusText0_0_4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cg := &ConfigGenerator{
				ScrapeOptions: tt.scrapeOptions,
			}
			got := cg.generateDefaultScrapeConfig()

			assert.Equal(t, model.Duration(tt.expectedInterval), got.ScrapeInterval)
			assert.Equal(t, model.Duration(tt.expectedTimeout), got.ScrapeTimeout)
			assert.Equal(t, tt.expectedFallbackProtocol, got.ScrapeFallbackProtocol)
			assert.Equal(t, tt.scrapeOptions.DefaultSampleLimit, got.SampleLimit)
		})
	}
}

func TestRelabelerAdd(t *testing.T) {
	relabeler := &relabeler{}

	cfgs := []*relabel.Config{
		{
			Action:      "",
			Separator:   "",
			Regex:       relabel.Regexp{},
			Replacement: "",
		},
	}
	expected := relabel.Config{
		Action:      relabel.DefaultRelabelConfig.Action,
		Separator:   relabel.DefaultRelabelConfig.Separator,
		Regex:       relabel.DefaultRelabelConfig.Regex,
		Replacement: relabel.DefaultRelabelConfig.Replacement,
	}
	relabeler.add(cfgs...)
	cfg := cfgs[0]

	assert.Equal(t, expected.Action, cfg.Action)
	assert.Equal(t, expected.Separator, cfg.Separator)
	assert.Equal(t, expected.Regex, cfg.Regex)
	assert.Equal(t, expected.Replacement, cfg.Replacement)
}

func TestRelabelerAddFromV1(t *testing.T) {
	relabeler := &relabeler{}

	cfgs := []promopv1.RelabelConfig{
		{
			SourceLabels: []promopv1.LabelName{"__meta_kubernetes_pod_label_app"},
			Separator:    stringPtr(";"),
			TargetLabel:  "app",
			Regex:        "(.*)",
			Modulus:      1,
			Replacement:  stringPtr("$1"),
			Action:       "replace",
		},
	}
	expected := relabel.Config{
		SourceLabels: model.LabelNames{"__meta_kubernetes_pod_label_app"},
		Separator:    ";",
		TargetLabel:  "app",
		Regex:        relabel.MustNewRegexp("(.*)"),
		Modulus:      1,
		Replacement:  "$1",
		Action:       relabel.Replace,
	}
	relabeler.addFromV1(cfgs...)
	cfg := relabeler.configs[0]

	assert.Equal(t, expected.SourceLabels, cfg.SourceLabels)
	assert.Equal(t, expected.Separator, cfg.Separator)
	assert.Equal(t, expected.TargetLabel, cfg.TargetLabel)
	assert.Equal(t, expected.Regex, cfg.Regex)
	assert.Equal(t, expected.Modulus, cfg.Modulus)
	assert.Equal(t, expected.Replacement, cfg.Replacement)
	assert.Equal(t, expected.Action, cfg.Action)
}

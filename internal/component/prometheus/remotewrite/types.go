package remotewrite

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	types "github.com/grafana/alloy/internal/component/common/config"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/syntax/alloytypes"

	"github.com/google/uuid"
	common "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	promsigv4 "github.com/prometheus/common/sigv4"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote/azuread"
)

// Defaults for config blocks.
var (
	DefaultArguments = Arguments{
		WALOptions: DefaultWALOptions,
	}

	DefaultQueueOptions = QueueOptions{
		Capacity:          10000,
		MaxShards:         50,
		MinShards:         1,
		MaxSamplesPerSend: 2000,
		BatchSendDeadline: 5 * time.Second,
		MinBackoff:        30 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		RetryOnHTTP429:    true,
		SampleAgeLimit:    0,
	}

	DefaultMetadataOptions = MetadataOptions{
		Send:              true,
		SendInterval:      1 * time.Minute,
		MaxSamplesPerSend: 2000,
	}

	DefaultWALOptions = WALOptions{
		TruncateFrequency: 2 * time.Hour,
		MinKeepaliveTime:  5 * time.Minute,
		MaxKeepaliveTime:  8 * time.Hour,
	}
)

// Arguments represents the input state of the prometheus.remote_write
// component.
type Arguments struct {
	ExternalLabels map[string]string  `alloy:"external_labels,attr,optional"`
	Endpoints      []*EndpointOptions `alloy:"endpoint,block,optional"`
	WALOptions     WALOptions         `alloy:"wal,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (rc *Arguments) SetToDefault() {
	*rc = DefaultArguments
}

// EndpointOptions describes an individual location for where metrics in the WAL
// should be delivered to using the remote_write protocol.
type EndpointOptions struct {
	Name                 string                  `alloy:"name,attr,optional"`
	URL                  string                  `alloy:"url,attr"`
	RemoteTimeout        time.Duration           `alloy:"remote_timeout,attr,optional"`
	Headers              map[string]string       `alloy:"headers,attr,optional"`
	SendExemplars        bool                    `alloy:"send_exemplars,attr,optional"`
	SendNativeHistograms bool                    `alloy:"send_native_histograms,attr,optional"`
	HTTPClientConfig     *types.HTTPClientConfig `alloy:",squash"`
	QueueOptions         *QueueOptions           `alloy:"queue_config,block,optional"`
	MetadataOptions      *MetadataOptions        `alloy:"metadata_config,block,optional"`
	WriteRelabelConfigs  []*alloy_relabel.Config `alloy:"write_relabel_config,block,optional"`
	SigV4                *SigV4Config            `alloy:"sigv4,block,optional"`
	AzureAD              *AzureADConfig          `alloy:"azuread,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	*r = EndpointOptions{
		RemoteTimeout:    30 * time.Second,
		SendExemplars:    true,
		HTTPClientConfig: types.CloneDefaultHTTPClientConfig(),
	}
}

func isAuthSetInHttpClientConfig(cfg *types.HTTPClientConfig) bool {
	return cfg.BasicAuth != nil ||
		cfg.OAuth2 != nil ||
		cfg.Authorization != nil ||
		len(cfg.BearerToken) > 0 ||
		len(cfg.BearerTokenFile) > 0
}

// Validate implements syntax.Validator.
func (r *EndpointOptions) Validate() error {
	// We must explicitly Validate because HTTPClientConfig is squashed and it won't run otherwise
	if r.HTTPClientConfig != nil {
		if err := r.HTTPClientConfig.Validate(); err != nil {
			return err
		}
	}

	const tooManyAuthErr = "at most one of sigv4, azuread, basic_auth, oauth2, bearer_token & bearer_token_file must be configured"

	if r.SigV4 != nil {
		if r.AzureAD != nil || isAuthSetInHttpClientConfig(r.HTTPClientConfig) {
			return fmt.Errorf(tooManyAuthErr)
		}
	}

	if r.AzureAD != nil {
		if r.SigV4 != nil || isAuthSetInHttpClientConfig(r.HTTPClientConfig) {
			return fmt.Errorf(tooManyAuthErr)
		}
	}

	if r.WriteRelabelConfigs != nil {
		for _, relabelConfig := range r.WriteRelabelConfigs {
			if err := relabelConfig.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// QueueOptions handles the low level queue config options for a remote_write
type QueueOptions struct {
	Capacity          int           `alloy:"capacity,attr,optional"`
	MaxShards         int           `alloy:"max_shards,attr,optional"`
	MinShards         int           `alloy:"min_shards,attr,optional"`
	MaxSamplesPerSend int           `alloy:"max_samples_per_send,attr,optional"`
	BatchSendDeadline time.Duration `alloy:"batch_send_deadline,attr,optional"`
	MinBackoff        time.Duration `alloy:"min_backoff,attr,optional"`
	MaxBackoff        time.Duration `alloy:"max_backoff,attr,optional"`
	RetryOnHTTP429    bool          `alloy:"retry_on_http_429,attr,optional"`
	SampleAgeLimit    time.Duration `alloy:"sample_age_limit,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *QueueOptions) SetToDefault() {
	*r = DefaultQueueOptions
}

func (r *QueueOptions) toPrometheusType() config.QueueConfig {
	if r == nil {
		var res QueueOptions
		res.SetToDefault()
		return res.toPrometheusType()
	}

	return config.QueueConfig{
		Capacity:          r.Capacity,
		MaxShards:         r.MaxShards,
		MinShards:         r.MinShards,
		MaxSamplesPerSend: r.MaxSamplesPerSend,
		BatchSendDeadline: model.Duration(r.BatchSendDeadline),
		MinBackoff:        model.Duration(r.MinBackoff),
		MaxBackoff:        model.Duration(r.MaxBackoff),
		RetryOnRateLimit:  r.RetryOnHTTP429,
		SampleAgeLimit:    model.Duration(r.SampleAgeLimit),
	}
}

// MetadataOptions configures how metadata gets sent over the remote_write
// protocol.
type MetadataOptions struct {
	Send              bool          `alloy:"send,attr,optional"`
	SendInterval      time.Duration `alloy:"send_interval,attr,optional"`
	MaxSamplesPerSend int           `alloy:"max_samples_per_send,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (o *MetadataOptions) SetToDefault() {
	*o = DefaultMetadataOptions
}

func (o *MetadataOptions) toPrometheusType() config.MetadataConfig {
	if o == nil {
		var res MetadataOptions
		res.SetToDefault()
		return res.toPrometheusType()
	}

	return config.MetadataConfig{
		Send:              o.Send,
		SendInterval:      model.Duration(o.SendInterval),
		MaxSamplesPerSend: o.MaxSamplesPerSend,
	}
}

// WALOptions configures behavior within the WAL.
type WALOptions struct {
	TruncateFrequency time.Duration `alloy:"truncate_frequency,attr,optional"`
	MinKeepaliveTime  time.Duration `alloy:"min_keepalive_time,attr,optional"`
	MaxKeepaliveTime  time.Duration `alloy:"max_keepalive_time,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (o *WALOptions) SetToDefault() {
	*o = DefaultWALOptions
}

// Validate implements syntax.Validator.
func (o *WALOptions) Validate() error {
	switch {
	case o.TruncateFrequency == 0:
		return fmt.Errorf("truncate_frequency must not be 0")
	case o.MaxKeepaliveTime <= o.MinKeepaliveTime:
		return fmt.Errorf("min_keepalive_time must be smaller than max_keepalive_time")
	}

	return nil
}

// Exports are the set of fields exposed by the prometheus.remote_write
// component.
type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

func convertConfigs(cfg Arguments) (*config.Config, error) {
	var rwConfigs []*config.RemoteWriteConfig
	for _, rw := range cfg.Endpoints {
		parsedURL, err := url.Parse(rw.URL)
		if err != nil {
			return nil, fmt.Errorf("cannot parse remote_write url %q: %w", rw.URL, err)
		}
		rwConfigs = append(rwConfigs, &config.RemoteWriteConfig{
			URL:                  &common.URL{URL: parsedURL},
			RemoteTimeout:        model.Duration(rw.RemoteTimeout),
			Headers:              rw.Headers,
			Name:                 rw.Name,
			SendExemplars:        rw.SendExemplars,
			SendNativeHistograms: rw.SendNativeHistograms,

			WriteRelabelConfigs: alloy_relabel.ComponentToPromRelabelConfigs(rw.WriteRelabelConfigs),
			HTTPClientConfig:    *rw.HTTPClientConfig.Convert(),
			QueueConfig:         rw.QueueOptions.toPrometheusType(),
			MetadataConfig:      rw.MetadataOptions.toPrometheusType(),
			SigV4Config:         rw.SigV4.toPrometheusType(),
			AzureADConfig:       rw.AzureAD.toPrometheusType(),
		})
	}

	return &config.Config{
		GlobalConfig: config.GlobalConfig{
			ExternalLabels: toLabels(cfg.ExternalLabels),
		},
		RemoteWriteConfigs: rwConfigs,
	}, nil
}

func toLabels(in map[string]string) labels.Labels {
	res := make(labels.Labels, 0, len(in))
	for k, v := range in {
		res = append(res, labels.Label{Name: k, Value: v})
	}
	sort.Sort(res)
	return res
}

// ManagedIdentityConfig is used to store managed identity config values
type ManagedIdentityConfig struct {
	// ClientID is the clientId of the managed identity that is being used to authenticate.
	ClientID string `alloy:"client_id,attr"`
}

func (m ManagedIdentityConfig) toPrometheusType() *azuread.ManagedIdentityConfig {
	if m.ClientID == "" {
		return nil
	}

	return &azuread.ManagedIdentityConfig{
		ClientID: m.ClientID,
	}
}

// Azure AD oauth
type AzureOAuthConfig struct {
	// AzureADOAuth is the OAuth configuration that is being used to authenticate.
	ClientID     string `alloy:"client_id,attr"`
	ClientSecret string `alloy:"client_secret,attr"`
	TenantID     string `alloy:"tenant_id,attr"`
}

func (m AzureOAuthConfig) toPrometheusType() *azuread.OAuthConfig {
	if m.ClientID == "" && m.ClientSecret == "" && m.TenantID == "" {
		return nil
	}

	return &azuread.OAuthConfig{
		ClientID:     m.ClientID,
		ClientSecret: m.ClientSecret,
		TenantID:     m.TenantID,
	}
}

type AzureADConfig struct {
	// ManagedIdentity is the managed identity that is being used to authenticate.
	ManagedIdentity ManagedIdentityConfig `alloy:"managed_identity,block,optional"`
	// OAuth is the OAuth configuration that is being used to authenticate.
	OAuth AzureOAuthConfig `alloy:"oauth,block,optional"`

	// Cloud is the Azure cloud in which the service is running. Example: AzurePublic/AzureGovernment/AzureChina.
	Cloud string `alloy:"cloud,attr,optional"`
}

func (a *AzureADConfig) Validate() error {
	if a.Cloud != azuread.AzureChina && a.Cloud != azuread.AzureGovernment && a.Cloud != azuread.AzurePublic {
		return fmt.Errorf("must provide a cloud in the Azure AD config")
	}

	_, err := uuid.Parse(a.ManagedIdentity.ClientID)
	if err != nil {
		return fmt.Errorf("the provided Azure Managed Identity client_id provided is invalid")
	}

	// Validate OAuth if it is provided
	// if a.OAuth != "" {
	// 	if a.OAuth.TenantID == "" {
	// 		return fmt.Errorf("OAuth TenantID must not be empty")
	// 	}
	// 	if a.OAuth.ClientSecret == "" {
	// 		return fmt.Errorf("OAuth ClientSecret must not be empty")
	// 	}
	// }

	// // Validate ManagedIdentity if it is provided
	// if a.ManagedIdentity != nil {
	// 	if a.ManagedIdentity.ClientID == "" {
	// 		return fmt.Errorf("ManagedIdentity ClientID must not be empty")
	// 	}
	// }

	// // Validate OAuth if it is provided
	// if a.OAuth != nil {
	// 	if a.OAuth.ClientID == "" {
	// 		return fmt.Errorf("OAuth ClientID must not be empty")
	// 	}
	// 	if a.OAuth.TenantID == "" {
	// 		return fmt.Errorf("OAuth TenantID must not be empty")
	// 	}
	// 	if a.OAuth.ClientSecret == "" {
	// 		return fmt.Errorf("OAuth ClientSecret must not be empty")
	// 	}
	// }

	return nil
}

// SetToDefault implements syntax.Defaulter.
func (a *AzureADConfig) SetToDefault() {
	*a = AzureADConfig{
		Cloud: azuread.AzurePublic,
	}
}

func (a *AzureADConfig) toPrometheusType() *azuread.AzureADConfig {
	if a == nil {
		return nil
	}

	mangedIdentity := a.ManagedIdentity.toPrometheusType()
	oauth := a.OAuth.toPrometheusType()
	return &azuread.AzureADConfig{
		OAuth:           oauth,
		ManagedIdentity: mangedIdentity,
		Cloud:           a.Cloud,
	}
}

type SigV4Config struct {
	Region    string            `alloy:"region,attr,optional"`
	AccessKey string            `alloy:"access_key,attr,optional"`
	SecretKey alloytypes.Secret `alloy:"secret_key,attr,optional"`
	Profile   string            `alloy:"profile,attr,optional"`
	RoleARN   string            `alloy:"role_arn,attr,optional"`
}

func (s *SigV4Config) Validate() error {
	if (s.AccessKey == "") != (s.SecretKey == "") {
		return fmt.Errorf("must provide an AWS SigV4 access key and secret key if credentials are specified in the SigV4 config")
	}
	return nil
}

func (s *SigV4Config) toPrometheusType() *promsigv4.SigV4Config {
	if s == nil {
		return nil
	}

	return &promsigv4.SigV4Config{
		Region:    s.Region,
		AccessKey: s.AccessKey,
		SecretKey: common.Secret(s.SecretKey),
		Profile:   s.Profile,
		RoleARN:   s.RoleARN,
	}
}

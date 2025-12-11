package remotewrite

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	types "github.com/grafana/alloy/internal/component/common/config"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/syntax/alloytypes"

	"github.com/google/uuid"
	"github.com/grafana/regexp"
	"github.com/prometheus/client_golang/exp/api/remote"
	common "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/prometheus/prometheus/storage/remote/azuread"
	promsigv4 "github.com/prometheus/sigv4"
)

// Defaults for config blocks.
var (
	PrometheusProtobufMessageV1 = string(remote.WriteV1MessageType)
	PrometheusProtobufMessageV2 = string(remote.WriteV2MessageType)

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

	errTooManyAuth = errors.New("at most one of sigv4, azuread, basic_auth, oauth2, bearer_token & bearer_token_file must be configured")
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
	ProtobufMessage      string                  `alloy:"protobuf_message,attr,optional"`
	HTTPClientConfig     *types.HTTPClientConfig `alloy:",squash"`
	QueueOptions         *QueueOptions           `alloy:"queue_config,block,optional"`
	MetadataOptions      *MetadataOptions        `alloy:"metadata_config,block,optional"`
	WriteRelabelConfigs  []*alloy_relabel.Config `alloy:"write_relabel_config,block,optional"`
	SigV4                *SigV4Config            `alloy:"sigv4,block,optional"`
	AzureAD              *AzureADConfig          `alloy:"azuread,block,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (r *EndpointOptions) SetToDefault() {
	defaultHTTPClientConfig := types.CloneDefaultHTTPClientConfig()
	defaultHTTPClientConfig.EnableHTTP2 = false // This has changed to false when we upgraded to Prometheus v3.4.2
	*r = EndpointOptions{
		RemoteTimeout:    30 * time.Second,
		SendExemplars:    true,
		ProtobufMessage:  PrometheusProtobufMessageV1,
		HTTPClientConfig: defaultHTTPClientConfig,
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

	if r.SigV4 != nil {
		if r.AzureAD != nil || isAuthSetInHttpClientConfig(r.HTTPClientConfig) {
			return errTooManyAuth
		}
	}

	if r.AzureAD != nil {
		if r.SigV4 != nil || isAuthSetInHttpClientConfig(r.HTTPClientConfig) {
			return errTooManyAuth
		}
	}

	if r.WriteRelabelConfigs != nil {
		for _, relabelConfig := range r.WriteRelabelConfigs {
			if err := relabelConfig.Validate(); err != nil {
				return err
			}
		}
	}

	if err := remote.WriteMessageType(r.ProtobufMessage).Validate(); err != nil {
		return fmt.Errorf("invalid protobuf_message %q for endpoint %q: %w", r.ProtobufMessage, r.Name, err)
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
		ProtobufMessage:      remote.WriteMessageType(rw.ProtobufMessage),
		WriteRelabelConfigs:  alloy_relabel.ComponentToPromRelabelConfigsLegacy(rw.WriteRelabelConfigs),
			HTTPClientConfig:     *rw.HTTPClientConfig.Convert(),
			QueueConfig:          rw.QueueOptions.toPrometheusType(),
			MetadataConfig:       rw.MetadataOptions.toPrometheusType(),
			SigV4Config:          rw.SigV4.toPrometheusType(),
			AzureADConfig:        rw.AzureAD.toPrometheusType(),
		})
	}

	return &config.Config{
		GlobalConfig: config.GlobalConfig{
			ExternalLabels: labels.FromMap(cfg.ExternalLabels),
		},
		RemoteWriteConfigs: rwConfigs,
	}, nil
}

// ManagedIdentityConfig is used to store managed identity config values
type ManagedIdentityConfig struct {
	// ClientID is the clientId of the managed identity that is being used to authenticate.
	ClientID string `alloy:"client_id,attr"`
}

func (m *ManagedIdentityConfig) toPrometheusType() *azuread.ManagedIdentityConfig {
	if m == nil {
		return nil
	}

	return &azuread.ManagedIdentityConfig{
		ClientID: m.ClientID,
	}
}

// OAuthConfig is used to store azure oauth config values.
type OAuthConfig struct {
	// ClientID is the clientId of the azure active directory application that is being used to authenticate.
	ClientID string `alloy:"client_id,attr"`

	// ClientSecret is the clientSecret of the azure active directory application that is being used to authenticate.
	ClientSecret alloytypes.Secret `alloy:"client_secret,attr"`

	// TenantID is the tenantId of the azure active directory application that is being used to authenticate.
	TenantID string `alloy:"tenant_id,attr"`
}

func (c *OAuthConfig) toPrometheusType() *azuread.OAuthConfig {
	if c == nil {
		return nil
	}

	return &azuread.OAuthConfig{
		ClientID: c.ClientID,
		// TODO(ptodev): Upstream a change to make this an opaque string.
		ClientSecret: string(c.ClientSecret),
		TenantID:     c.TenantID,
	}
}

// SDKConfig is used to store azure SDK config values.
type SDKConfig struct {
	// TenantID is the tenantId of the azure active directory application that is being used to authenticate.
	TenantID string `alloy:"tenant_id,attr"`
}

func (c *SDKConfig) toPrometheusType() *azuread.SDKConfig {
	if c == nil {
		return nil
	}

	return &azuread.SDKConfig{
		TenantID: c.TenantID,
	}
}

type AzureADConfig struct {
	// ManagedIdentity is the managed identity that is being used to authenticate.
	ManagedIdentity *ManagedIdentityConfig `alloy:"managed_identity,block,optional"`

	// OAuth is the oauth config that is being used to authenticate.
	OAuth *OAuthConfig `alloy:"oauth,block,optional"`

	// SDK is the SDK config that is being used to authenticate.
	SDK *SDKConfig `alloy:"sdk,block,optional"`

	// Cloud is the Azure cloud in which the service is running. Example: AzurePublic/AzureGovernment/AzureChina.
	Cloud string `alloy:"cloud,attr,optional"`
}

func (a *AzureADConfig) Validate() error {
	if a.Cloud != azuread.AzureChina && a.Cloud != azuread.AzureGovernment && a.Cloud != azuread.AzurePublic {
		return fmt.Errorf("must provide a cloud in the Azure AD config")
	}

	if a.ManagedIdentity == nil && a.OAuth == nil && a.SDK == nil {
		return fmt.Errorf("must provide an Azure Managed Identity, Azure OAuth or Azure SDK in the Azure AD config")
	}

	if a.ManagedIdentity != nil && a.OAuth != nil {
		return fmt.Errorf("cannot provide both Azure Managed Identity and Azure OAuth in the Azure AD config")
	}

	if a.ManagedIdentity != nil && a.SDK != nil {
		return fmt.Errorf("cannot provide both Azure Managed Identity and Azure SDK in the Azure AD config")
	}

	if a.OAuth != nil && a.SDK != nil {
		return fmt.Errorf("cannot provide both Azure OAuth and Azure SDK in the Azure AD config")
	}

	if a.ManagedIdentity != nil {
		if a.ManagedIdentity.ClientID == "" {
			return fmt.Errorf("must provide an Azure Managed Identity client_id in the Azure AD config")
		}

		_, err := uuid.Parse(a.ManagedIdentity.ClientID)
		if err != nil {
			return fmt.Errorf("the provided Azure Managed Identity client_id is invalid")
		}
	}

	if a.OAuth != nil {
		if a.OAuth.ClientID == "" {
			return fmt.Errorf("must provide an Azure OAuth client_id in the Azure AD config")
		}
		if a.OAuth.ClientSecret == "" {
			return fmt.Errorf("must provide an Azure OAuth client_secret in the Azure AD config")
		}
		if a.OAuth.TenantID == "" {
			return fmt.Errorf("must provide an Azure OAuth tenant_id in the Azure AD config")
		}

		var err error
		_, err = uuid.Parse(a.OAuth.ClientID)
		if err != nil {
			return fmt.Errorf("the provided Azure OAuth client_id is invalid")
		}
		_, err = regexp.MatchString("^[0-9a-zA-Z-.]+$", a.OAuth.TenantID)
		if err != nil {
			return fmt.Errorf("the provided Azure OAuth tenant_id is invalid")
		}
	}

	if a.SDK != nil {
		var err error

		if a.SDK.TenantID != "" {
			_, err = regexp.MatchString("^[0-9a-zA-Z-.]+$", a.SDK.TenantID)
			if err != nil {
				return fmt.Errorf("the provided Azure OAuth tenant_id is invalid")
			}
		}
	}

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

	return &azuread.AzureADConfig{
		ManagedIdentity: a.ManagedIdentity.toPrometheusType(),
		OAuth:           a.OAuth.toPrometheusType(),
		SDK:             a.SDK.toPrometheusType(),
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

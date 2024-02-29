package remotewrite

import (
	"fmt"
	"net/url"
	"sort"
	"time"

	types "github.com/grafana/agent/internal/component/common/config"
	flow_relabel "github.com/grafana/agent/internal/component/common/relabel"
	"github.com/grafana/river/rivertypes"

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
	ExternalLabels map[string]string  `river:"external_labels,attr,optional"`
	Endpoints      []*EndpointOptions `river:"endpoint,block,optional"`
	WALOptions     WALOptions         `river:"wal,block,optional"`
}

// SetToDefault implements river.Defaulter.
func (rc *Arguments) SetToDefault() {
	*rc = DefaultArguments
}

// EndpointOptions describes an individual location for where metrics in the WAL
// should be delivered to using the remote_write protocol.
type EndpointOptions struct {
	Name                 string                  `river:"name,attr,optional"`
	URL                  string                  `river:"url,attr"`
	RemoteTimeout        time.Duration           `river:"remote_timeout,attr,optional"`
	Headers              map[string]string       `river:"headers,attr,optional"`
	SendExemplars        bool                    `river:"send_exemplars,attr,optional"`
	SendNativeHistograms bool                    `river:"send_native_histograms,attr,optional"`
	HTTPClientConfig     *types.HTTPClientConfig `river:",squash"`
	QueueOptions         *QueueOptions           `river:"queue_config,block,optional"`
	MetadataOptions      *MetadataOptions        `river:"metadata_config,block,optional"`
	WriteRelabelConfigs  []*flow_relabel.Config  `river:"write_relabel_config,block,optional"`
	SigV4                *SigV4Config            `river:"sigv4,block,optional"`
	AzureAD              *AzureADConfig          `river:"azuread,block,optional"`
}

// SetToDefault implements river.Defaulter.
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

// Validate implements river.Validator.
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
	Capacity          int           `river:"capacity,attr,optional"`
	MaxShards         int           `river:"max_shards,attr,optional"`
	MinShards         int           `river:"min_shards,attr,optional"`
	MaxSamplesPerSend int           `river:"max_samples_per_send,attr,optional"`
	BatchSendDeadline time.Duration `river:"batch_send_deadline,attr,optional"`
	MinBackoff        time.Duration `river:"min_backoff,attr,optional"`
	MaxBackoff        time.Duration `river:"max_backoff,attr,optional"`
	RetryOnHTTP429    bool          `river:"retry_on_http_429,attr,optional"`
	SampleAgeLimit    time.Duration `river:"sample_age_limit,attr,optional"`
}

// SetToDefault implements river.Defaulter.
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
	Send              bool          `river:"send,attr,optional"`
	SendInterval      time.Duration `river:"send_interval,attr,optional"`
	MaxSamplesPerSend int           `river:"max_samples_per_send,attr,optional"`
}

// SetToDefault implements river.Defaulter.
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
	TruncateFrequency time.Duration `river:"truncate_frequency,attr,optional"`
	MinKeepaliveTime  time.Duration `river:"min_keepalive_time,attr,optional"`
	MaxKeepaliveTime  time.Duration `river:"max_keepalive_time,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (o *WALOptions) SetToDefault() {
	*o = DefaultWALOptions
}

// Validate implements river.Validator.
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
	Receiver storage.Appendable `river:"receiver,attr"`
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

			WriteRelabelConfigs: flow_relabel.ComponentToPromRelabelConfigs(rw.WriteRelabelConfigs),
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
	ClientID string `river:"client_id,attr"`
}

func (m ManagedIdentityConfig) toPrometheusType() azuread.ManagedIdentityConfig {
	return azuread.ManagedIdentityConfig{
		ClientID: m.ClientID,
	}
}

type AzureADConfig struct {
	// ManagedIdentity is the managed identity that is being used to authenticate.
	ManagedIdentity ManagedIdentityConfig `river:"managed_identity,block"`

	// Cloud is the Azure cloud in which the service is running. Example: AzurePublic/AzureGovernment/AzureChina.
	Cloud string `river:"cloud,attr,optional"`
}

func (a *AzureADConfig) Validate() error {
	if a.Cloud != azuread.AzureChina && a.Cloud != azuread.AzureGovernment && a.Cloud != azuread.AzurePublic {
		return fmt.Errorf("must provide a cloud in the Azure AD config")
	}

	_, err := uuid.Parse(a.ManagedIdentity.ClientID)
	if err != nil {
		return fmt.Errorf("the provided Azure Managed Identity client_id provided is invalid")
	}

	return nil
}

// SetToDefault implements river.Defaulter.
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
	return &azuread.AzureADConfig{
		ManagedIdentity: &mangedIdentity,
		Cloud:           a.Cloud,
	}
}

type SigV4Config struct {
	Region    string            `river:"region,attr,optional"`
	AccessKey string            `river:"access_key,attr,optional"`
	SecretKey rivertypes.Secret `river:"secret_key,attr,optional"`
	Profile   string            `river:"profile,attr,optional"`
	RoleARN   string            `river:"role_arn,attr,optional"`
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

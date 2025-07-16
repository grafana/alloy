package component

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert/build"
	"github.com/grafana/alloy/syntax/alloytypes"
	prom_config "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/storage/remote/azuread"
	"github.com/prometheus/sigv4"
)

func AppendPrometheusRemoteWrite(pb *build.PrometheusBlocks, globalConfig prom_config.GlobalConfig, remoteWriteConfigs []*prom_config.RemoteWriteConfig, label string) *remotewrite.Exports {
	remoteWriteArgs := toRemotewriteArguments(globalConfig, remoteWriteConfigs)

	remoteWriteLabel := label
	if remoteWriteLabel == "" {
		remoteWriteLabel = "default"
	}

	if len(remoteWriteConfigs) > 0 {
		name := []string{"prometheus", "remote_write"}
		block := common.NewBlockWithOverride(name, remoteWriteLabel, remoteWriteArgs)

		names := []string{}
		for _, remoteWriteConfig := range remoteWriteConfigs {
			names = append(names, remoteWriteConfig.Name)
		}
		summary := fmt.Sprintf("Converted %d remote_write[s] %q into...", len(remoteWriteConfigs), strings.Join(names, ","))
		detail := fmt.Sprintf("	A prometheus.remote_write.%s component", remoteWriteLabel)
		pb.PrometheusRemoteWriteBlocks = append(pb.PrometheusRemoteWriteBlocks, build.NewPrometheusBlock(block, name, remoteWriteLabel, summary, detail))
	}

	return &remotewrite.Exports{
		Receiver: common.ConvertAppendable{Expr: "prometheus.remote_write." + remoteWriteLabel + ".receiver"},
	}
}

func ValidateRemoteWriteConfig(remoteWriteConfig *prom_config.RemoteWriteConfig) diag.Diagnostics {
	var diags diag.Diagnostics

	diags.AddAll(common.ValidateHttpClientConfig(&remoteWriteConfig.HTTPClientConfig))
	return diags
}

func toRemotewriteArguments(globalConfig prom_config.GlobalConfig, remoteWriteConfigs []*prom_config.RemoteWriteConfig) *remotewrite.Arguments {
	externalLabels := globalConfig.ExternalLabels.Map()
	if len(externalLabels) == 0 {
		externalLabels = nil
	}

	return &remotewrite.Arguments{
		ExternalLabels: externalLabels,
		Endpoints:      getEndpointOptions(remoteWriteConfigs),
		WALOptions:     remotewrite.DefaultWALOptions,
	}
}

func getEndpointOptions(remoteWriteConfigs []*prom_config.RemoteWriteConfig) []*remotewrite.EndpointOptions {
	endpoints := make([]*remotewrite.EndpointOptions, 0)

	for _, remoteWriteConfig := range remoteWriteConfigs {
		endpoint := &remotewrite.EndpointOptions{
			Name:                 remoteWriteConfig.Name,
			URL:                  remoteWriteConfig.URL.String(),
			RemoteTimeout:        time.Duration(remoteWriteConfig.RemoteTimeout),
			Headers:              remoteWriteConfig.Headers,
			SendExemplars:        remoteWriteConfig.SendExemplars,
			SendNativeHistograms: remoteWriteConfig.SendNativeHistograms,
			ProtobufMessage:      string(remoteWriteConfig.ProtobufMessage),
			HTTPClientConfig:     common.ToHttpClientConfig(&remoteWriteConfig.HTTPClientConfig),
			QueueOptions:         toQueueOptions(&remoteWriteConfig.QueueConfig),
			MetadataOptions:      toMetadataOptions(&remoteWriteConfig.MetadataConfig),
			WriteRelabelConfigs:  ToAlloyRelabelConfigs(remoteWriteConfig.WriteRelabelConfigs),
			SigV4:                toSigV4(remoteWriteConfig.SigV4Config),
			AzureAD:              toAzureAD(remoteWriteConfig.AzureADConfig),
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}

func toQueueOptions(queueConfig *prom_config.QueueConfig) *remotewrite.QueueOptions {
	return &remotewrite.QueueOptions{
		Capacity:          queueConfig.Capacity,
		MaxShards:         queueConfig.MaxShards,
		MinShards:         queueConfig.MinShards,
		MaxSamplesPerSend: queueConfig.MaxSamplesPerSend,
		BatchSendDeadline: time.Duration(queueConfig.BatchSendDeadline),
		MinBackoff:        time.Duration(queueConfig.MinBackoff),
		MaxBackoff:        time.Duration(queueConfig.MaxBackoff),
		RetryOnHTTP429:    queueConfig.RetryOnRateLimit,
		SampleAgeLimit:    time.Duration(queueConfig.SampleAgeLimit),
	}
}

func toMetadataOptions(metadataConfig *prom_config.MetadataConfig) *remotewrite.MetadataOptions {
	return &remotewrite.MetadataOptions{
		Send:              metadataConfig.Send,
		SendInterval:      time.Duration(metadataConfig.SendInterval),
		MaxSamplesPerSend: metadataConfig.MaxSamplesPerSend,
	}
}

// toSigV4 converts a Prometheus SigV4 config to an Alloy SigV4 config.
func toSigV4(sigv4Config *sigv4.SigV4Config) *remotewrite.SigV4Config {
	if sigv4Config == nil {
		return nil
	}

	return &remotewrite.SigV4Config{
		Region:    sigv4Config.Region,
		AccessKey: sigv4Config.AccessKey,
		SecretKey: alloytypes.Secret(sigv4Config.SecretKey),
		Profile:   sigv4Config.Profile,
		RoleARN:   sigv4Config.RoleARN,
	}
}

// toAzureAD converts a Prometheus AzureAD config to an Alloy AzureAD config.
func toAzureAD(azureADConfig *azuread.AzureADConfig) *remotewrite.AzureADConfig {
	if azureADConfig == nil {
		return nil
	}

	res := &remotewrite.AzureADConfig{
		Cloud: azureADConfig.Cloud,
	}

	if azureADConfig.ManagedIdentity != nil {
		res.ManagedIdentity = &remotewrite.ManagedIdentityConfig{
			ClientID: azureADConfig.ManagedIdentity.ClientID,
		}
	}

	if azureADConfig.OAuth != nil {
		res.OAuth = &remotewrite.OAuthConfig{
			ClientID:     azureADConfig.OAuth.ClientID,
			ClientSecret: alloytypes.Secret(azureADConfig.OAuth.ClientSecret),
			TenantID:     azureADConfig.OAuth.TenantID,
		}
	}

	if azureADConfig.SDK != nil {
		res.SDK = &remotewrite.SDKConfig{
			TenantID: azureADConfig.SDK.TenantID,
		}
	}

	return res
}

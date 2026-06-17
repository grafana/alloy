package build

import (
	"reflect"

	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/static/server"
)

func (b *ConfigBuilder) appendServer(config *server.Config) {
	args := toServer(config)
	normalizeEmptyTLSWindowsFilter(args.TLS)
	if !reflect.DeepEqual(*args.TLS, http.TLSArguments{}) {
		b.f.Body().AppendBlock(common.NewBlockWithOverride(
			[]string{"http"},
			"",
			args,
		))
	}
}

func normalizeEmptyTLSWindowsFilter(tlsArgs *http.TLSArguments) {
	if tlsArgs == nil || tlsArgs.WindowsFilter == nil {
		return
	}

	tlsArgs.WindowsFilter.Server = normalizeEmptyWindowsServerFilter(tlsArgs.WindowsFilter.Server)
	tlsArgs.WindowsFilter.Client = normalizeEmptyWindowsClientFilter(tlsArgs.WindowsFilter.Client)
	if tlsArgs.WindowsFilter.Server == nil && tlsArgs.WindowsFilter.Client == nil {
		tlsArgs.WindowsFilter = nil
	}
}

func normalizeEmptyWindowsServerFilter(serverFilter *http.WindowsServerFilter) *http.WindowsServerFilter {
	if serverFilter == nil {
		return nil
	}
	if len(serverFilter.IssuerCommonNames) == 0 {
		serverFilter.IssuerCommonNames = nil
	}
	if serverFilter.Store == "" &&
		serverFilter.SystemStore == "" &&
		len(serverFilter.IssuerCommonNames) == 0 &&
		serverFilter.TemplateID == "" &&
		serverFilter.RefreshInterval == 0 {
		return nil
	}
	return serverFilter
}

func normalizeEmptyWindowsClientFilter(clientFilter *http.WindowsClientFilter) *http.WindowsClientFilter {
	if clientFilter == nil {
		return nil
	}
	if len(clientFilter.IssuerCommonNames) == 0 {
		clientFilter.IssuerCommonNames = nil
	}
	if len(clientFilter.IssuerCommonNames) == 0 &&
		clientFilter.SubjectRegEx == "" &&
		clientFilter.TemplateID == "" {
		return nil
	}
	return clientFilter
}

func toServer(config *server.Config) *http.Arguments {
	authType, err := server.GetClientAuthFromString(config.HTTP.TLSConfig.ClientAuth)
	if err != nil {
		panic(err)
	}

	return &http.Arguments{
		TLS: &http.TLSArguments{
			Cert:             "",
			CertFile:         config.HTTP.TLSConfig.TLSCertPath,
			Key:              "",
			KeyFile:          config.HTTP.TLSConfig.TLSKeyPath,
			ClientCA:         "",
			ClientCAFile:     config.HTTP.TLSConfig.ClientCAs,
			ClientAuth:       http.ClientAuth(authType),
			CipherSuites:     toHTTPTLSCipher(config.HTTP.TLSConfig.CipherSuites),
			CurvePreferences: toHTTPTLSCurve(config.HTTP.TLSConfig.CurvePreferences),
			MinVersion:       http.TLSVersion(config.HTTP.TLSConfig.MinVersion),
			MaxVersion:       http.TLSVersion(config.HTTP.TLSConfig.MaxVersion),
			WindowsFilter:    toWindowsFilter(config.HTTP.TLSConfig.WindowsCertificateFilter),
		},
	}
}

func toHTTPTLSCipher(cipherSuites []server.TLSCipher) []http.TLSCipher {
	var result []http.TLSCipher
	for _, cipcipherSuite := range cipherSuites {
		result = append(result, http.TLSCipher(cipcipherSuite))
	}

	return result
}

func toHTTPTLSCurve(curvePreferences []server.TLSCurve) []http.TLSCurve {
	var result []http.TLSCurve
	for _, curvePreference := range curvePreferences {
		result = append(result, http.TLSCurve(curvePreference))
	}

	return result
}

func toWindowsFilter(windowsFilter *server.WindowsCertificateFilter) *http.WindowsCertificateFilter {
	if windowsFilter == nil {
		return nil
	}

	return &http.WindowsCertificateFilter{
		Server: toWindowsServerFilter(windowsFilter.Server),
		Client: toWindowsClientFilter(windowsFilter.Client),
	}
}

func toWindowsServerFilter(server *server.WindowsServerFilter) *http.WindowsServerFilter {
	if server == nil {
		return nil
	}

	return &http.WindowsServerFilter{
		Store:             server.Store,
		SystemStore:       server.SystemStore,
		IssuerCommonNames: server.IssuerCommonNames,
		TemplateID:        server.TemplateID,
		RefreshInterval:   server.RefreshInterval,
	}
}

func toWindowsClientFilter(client *server.WindowsClientFilter) *http.WindowsClientFilter {
	if client == nil {
		return nil
	}

	return &http.WindowsClientFilter{
		IssuerCommonNames: client.IssuerCommonNames,
		SubjectRegEx:      client.SubjectRegEx,
		TemplateID:        client.TemplateID,
	}
}

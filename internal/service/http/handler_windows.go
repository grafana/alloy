package http

import (
	"crypto/tls"

	"github.com/grafana/agent/internal/static/server"
)

// tlsConfig generates a tls.Config from args.
func (args *TLSArguments) winTlsConfig(win *server.WinCertStoreHandler) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion:            uint16(args.MinVersion),
		MaxVersion:            uint16(args.MaxVersion),
		ClientAuth:            tls.ClientAuthType(args.ClientAuth),
		VerifyPeerCertificate: win.VerifyPeer,
		GetCertificate:        win.CertificateHandler,
	}

	for _, c := range args.CipherSuites {
		config.CipherSuites = append(config.CipherSuites, uint16(c))
	}
	for _, c := range args.CurvePreferences {
		config.CurvePreferences = append(config.CurvePreferences, tls.CurveID(c))
	}
	return config, nil
}

func (s *Service) updateWindowsCertificateFilter(tlsArgs *TLSArguments) error {
	s.winMut.Lock()
	defer s.winMut.Unlock()
	// Stop if Window Handler is currently running.
	if s.win != nil {
		s.win.Stop()
	}
	handler, err := server.NewWinCertStoreHandler(tlsArgs.WindowsFilter.toYaml(), tls.ClientAuthType(tlsArgs.ClientAuth), s.log)
	if err != nil {
		return err
	}
	s.win = handler
	s.win.Run()
	return nil
}

func (wcf *WindowsCertificateFilter) toYaml() server.WindowsCertificateFilter {
	return server.WindowsCertificateFilter{
		Server: &server.WindowsServerFilter{
			Store:             wcf.Server.Store,
			SystemStore:       wcf.Server.SystemStore,
			IssuerCommonNames: wcf.Server.IssuerCommonNames,
			TemplateID:        wcf.Server.TemplateID,
			RefreshInterval:   wcf.Server.RefreshInterval,
		},
		Client: &server.WindowsClientFilter{
			IssuerCommonNames: wcf.Client.IssuerCommonNames,
			SubjectRegEx:      wcf.Client.SubjectRegEx,
			TemplateID:        wcf.Client.TemplateID,
		},
	}
}

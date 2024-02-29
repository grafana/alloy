package server

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/github/smimesign/certstore"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// Documentation on how to setup a certificate store can be found at docs/developer/windows

// WinCertStoreHandler handles the finding of certificates, validating them and injecting into the default TLS pipeline
type WinCertStoreHandler struct {
	cfg          WindowsCertificateFilter
	subjectRegEx *regexp.Regexp
	log          log.Logger

	winMut       sync.Mutex
	serverCert   *x509.Certificate
	serverSigner crypto.PrivateKey
	// We have to store the identity to access the signer (it's a win32 api call), if we close the identity
	// we lose access to the signer
	// the client does NOT need the signer
	serverIdentity certstore.Identity
	clientAuth     tls.ClientAuthType
	shutdown       chan struct{}
}

// NewWinCertStoreHandler creates a new WindowsCertificateFilter. Run should be called afterward.
func NewWinCertStoreHandler(cfg WindowsCertificateFilter, clientAuth tls.ClientAuthType, l log.Logger) (*WinCertStoreHandler, error) {
	var subjectRegEx *regexp.Regexp
	var err error
	if cfg.Client != nil && cfg.Client.SubjectRegEx != "" {
		subjectRegEx, err = regexp.Compile(cfg.Client.SubjectRegEx)
		if err != nil {
			return nil, fmt.Errorf("error compiling subject common name regular expression: %w", err)
		}
	}
	cn := &WinCertStoreHandler{
		cfg:          cfg,
		subjectRegEx: subjectRegEx,
		log:          l,
		shutdown:     make(chan struct{}),
	}
	err = cn.refreshCerts()
	if err != nil {
		return nil, err
	}
	cn.clientAuth = clientAuth
	return cn, nil
}

func (l *tlsListener) applyWindowsCertificateStore(c TLSConfig) error {

	// Restrict normal TLS options when using windows certificate store
	if c.TLSCertPath != "" {
		return fmt.Errorf("at most one of cert_file and windows_certificate_filter can be configured")
	}
	if c.TLSKeyPath != "" {
		return fmt.Errorf("at most one of cert_key and windows_certificate_filter can be configured")
	}
	if c.WindowsCertificateFilter.Server == nil {
		return fmt.Errorf("windows certificate filter requires a server block defined")
	}

	var subjectRegEx *regexp.Regexp
	var err error
	if c.WindowsCertificateFilter.Client != nil && c.WindowsCertificateFilter.Client.SubjectRegEx != "" {
		subjectRegEx, err = regexp.Compile(c.WindowsCertificateFilter.Client.SubjectRegEx)
		if err != nil {
			return fmt.Errorf("error compiling subject common name regular expression: %w", err)
		}
	}

	// If there is an existing windows certhandler stop it.
	if l.windowsCertHandler != nil {
		l.windowsCertHandler.Stop()
	}

	cn := &WinCertStoreHandler{
		cfg:          *c.WindowsCertificateFilter,
		subjectRegEx: subjectRegEx,
		log:          l.log,
		shutdown:     make(chan struct{}),
	}

	err = cn.refreshCerts()
	if err != nil {
		return err
	}

	config := &tls.Config{
		VerifyPeerCertificate: cn.VerifyPeer,
		GetCertificate:        cn.CertificateHandler,
		MaxVersion:            uint16(c.MaxVersion),
		MinVersion:            uint16(c.MinVersion),
	}

	ca, err := GetClientAuthFromString(c.ClientAuth)
	if err != nil {
		return err
	}
	config.ClientAuth = ca
	cn.clientAuth = ca
	// Kick off the refresh handler
	go cn.startUpdateTimer()
	l.windowsCertHandler = cn
	l.tlsConfig = config
	l.cfg = c
	return nil
}

// Run runs the filter refresh. Stop should be called when done.
func (c *WinCertStoreHandler) Run() {
	go c.startUpdateTimer()
}

// Stop shuts down the refresh loop and frees the win32 handles.
func (c *WinCertStoreHandler) Stop() {
	c.shutdown <- struct{}{}
}

// CertificateHandler returns the certificate to handle peer check.
func (c *WinCertStoreHandler) CertificateHandler(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.winMut.Lock()
	defer c.winMut.Unlock()

	cert := &tls.Certificate{
		Certificate: [][]byte{c.serverCert.Raw},
		PrivateKey:  c.serverSigner,
		Leaf:        c.serverCert,
	}
	return cert, nil
}

// VerifyPeer is called by the TLS pipeline, and specified in tls.config, this is where any custom verification happens
func (c *WinCertStoreHandler) VerifyPeer(_ [][]byte, verifiedChains [][]*x509.Certificate) error {
	opts := x509.VerifyOptions{}
	clientCert := verifiedChains[0][0]

	// Check for issuer
	issuerMatches := len(c.cfg.Client.IssuerCommonNames) == 0
	for _, cn := range c.cfg.Client.IssuerCommonNames {
		if cn == clientCert.Issuer.CommonName {
			issuerMatches = true
			break
		}
	}
	if !issuerMatches {
		return fmt.Errorf("unable to match client issuer")
	}

	// Check for subject
	subjectMatches := true
	if c.subjectRegEx != nil {
		if !c.subjectRegEx.MatchString(clientCert.Subject.CommonName) {
			subjectMatches = false
		}
	}
	if !subjectMatches {
		return fmt.Errorf("unable to match client subject")
	}

	// Check for template id
	if c.cfg.Client.TemplateID != "" {
		templateid := getTemplateID(clientCert)
		if templateid != c.cfg.Client.TemplateID {
			return fmt.Errorf("unable to match client template id")
		}
	}

	// call the normal pipeline
	_, err := clientCert.Verify(opts)
	return err
}

// this is the ASN1 Object Identifier for TemplateID
var asnTemplateOID = "1.3.6.1.4.1.311.21.7"

type templateInformation struct {
	Template     asn1.ObjectIdentifier
	MajorVersion int
	MinorVersion int
}

func (c *WinCertStoreHandler) startUpdateTimer() {
	refreshInterval := 5 * time.Minute
	c.winMut.Lock()
	if c.cfg.Server.RefreshInterval != 0 {
		refreshInterval = c.cfg.Server.RefreshInterval
	}
	c.winMut.Unlock()
	for {
		select {
		case <-c.shutdown:
			if c.serverIdentity != nil {
				c.serverIdentity.Close()
			}
			c.serverCert = nil
			c.serverSigner = nil
			return
		case <-time.After(refreshInterval):
			err := c.refreshCerts()
			if err != nil {
				level.Error(c.log).Log("msg", "error refreshing Windows certificates", "err", err)
			}

		}
	}
}

// refreshCerts is the main work item in certificate store, responsible for finding the right certificate
func (c *WinCertStoreHandler) refreshCerts() (err error) {
	c.winMut.Lock()
	defer c.winMut.Unlock()

	level.Debug(c.log).Log("msg", "refreshing Windows certificates")
	// Close the server identity if already set
	if c.serverIdentity != nil {
		c.serverIdentity.Close()
	}
	var serverIdentity certstore.Identity
	// This handles closing all our various handles
	defer func() {
		// we have to keep the server identity open if we want to use it, BUT only if an error occurred, else we need it
		// open to sign
		if serverIdentity != nil && err != nil {
			serverIdentity.Close()
		}
	}()
	serverIdentity, err = c.findServerIdentity()
	if err != nil {
		return fmt.Errorf("failed finding server identity %w", err)
	}
	sc, err := serverIdentity.Certificate()
	if err != nil {
		return fmt.Errorf("failed getting server certificate %w", err)
	}
	signer, err := serverIdentity.Signer()
	if err != nil {
		return fmt.Errorf("failed getting server signer %w", err)

	}

	c.serverCert = sc
	c.serverSigner = signer
	c.serverIdentity = serverIdentity
	return
}

func (c *WinCertStoreHandler) findServerIdentity() (certstore.Identity, error) {
	return c.findCertificate(c.cfg.Server.SystemStore, c.cfg.Server.Store, c.cfg.Server.IssuerCommonNames, c.cfg.Server.TemplateID, nil, c.getStore)
}

// getStore converts the string representation to the enum representation
func (c *WinCertStoreHandler) getStore(systemStore string, storeName string) (certstore.Store, error) {
	st, err := certstore.StringToStoreType(systemStore)
	if err != nil {
		return nil, err
	}
	store, err := certstore.OpenSpecificStore(st, storeName)
	if err != nil {
		return nil, err
	}
	return store, nil
}

type getStoreFunc func(systemStore, storeName string) (certstore.Store, error)

// findCertificate applies the filters to get the server certificate
func (c *WinCertStoreHandler) findCertificate(systemStore string, storeName string, commonNames []string, templateID string, subjectRegEx *regexp.Regexp, getStore getStoreFunc) (certstore.Identity, error) {
	var store certstore.Store
	var validIdentity certstore.Identity
	var identities []certstore.Identity
	// Lots of cleanup here for pointers to windows handles.
	defer func() {
		if store != nil {
			store.Close()
		}
		// Now lets clean up any identities that are NOT the one we want
		for _, i := range identities {
			if i != validIdentity {
				i.Close()
			}
		}
	}()
	store, err := getStore(systemStore, storeName)
	if err != nil {
		return nil, err
	}
	identities, err = store.Identities()

	filtered, err := c.filterByIssuerCommonNames(identities, commonNames)
	if err != nil {
		return nil, err
	}
	filtered, err = c.filterByTemplateID(filtered, templateID)
	if err != nil {
		return nil, err
	}
	filtered, err = c.filterBySubjectRegularExpression(filtered, subjectRegEx)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no certificates found")
	}
	// order oldest to newest
	sort.Slice(filtered, func(certI, certJ int) bool {
		// Already accessed this so the error will not happen
		a, _ := filtered[certI].Certificate()
		b, _ := filtered[certJ].Certificate()

		return a.NotBefore.Before(b.NotBefore)
	})

	// Grab the most recent valid one
	for i := len(filtered) - 1; i >= 0; i-- {
		fc, _ := filtered[i].Certificate()
		if time.Now().Before(fc.NotAfter) && time.Now().After(fc.NotBefore) {
			validIdentity = filtered[i]
			break
		}
	}
	if validIdentity == nil {
		return nil, fmt.Errorf("no certificates found")
	}

	return validIdentity, nil
}

func (c *WinCertStoreHandler) filterByIssuerCommonNames(input []certstore.Identity, commonNames []string) ([]certstore.Identity, error) {
	if len(commonNames) == 0 {
		return input, nil
	}
	returnIdentities := make([]certstore.Identity, 0)
	for _, identity := range input {
		cert, err := identity.Certificate()
		if err != nil {
			return nil, err
		}
		for _, cfgName := range commonNames {
			if cert.Issuer.CommonName == cfgName {
				returnIdentities = append(returnIdentities, identity)
				break
			}
		}
	}
	return returnIdentities, nil
}

func (c *WinCertStoreHandler) filterByTemplateID(input []certstore.Identity, id string) ([]certstore.Identity, error) {
	if id == "" {
		return input, nil
	}
	returnIdentities := make([]certstore.Identity, 0)

	for _, identity := range input {
		cert, err := identity.Certificate()
		if err != nil {
			return nil, err
		}
		templateid := getTemplateID(cert)
		if templateid == id {
			returnIdentities = append(returnIdentities, identity)
		}
	}
	return returnIdentities, nil
}

func getTemplateID(cert *x509.Certificate) string {
	for _, ext := range cert.Extensions {
		if ext.Id.String() == asnTemplateOID {
			templateInfo := &templateInformation{}
			_, err := asn1.Unmarshal(ext.Value, templateInfo)
			if err != nil {
				return ""
			}
			return templateInfo.Template.String()
		}
	}
	return ""
}

func (c *WinCertStoreHandler) filterBySubjectRegularExpression(input []certstore.Identity, regEx *regexp.Regexp) ([]certstore.Identity, error) {
	if regEx == nil {
		return input, nil
	}
	returnIdentities := make([]certstore.Identity, 0)

	for _, identity := range input {
		cert, err := identity.Certificate()
		if err != nil {
			return nil, err
		}
		if regEx.MatchString(cert.Subject.CommonName) {
			returnIdentities = append(returnIdentities, identity)
		}
	}
	return returnIdentities, nil
}

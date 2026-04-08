// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package promhttp2

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	commonconfig "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	uberatomic "go.uber.org/atomic"
	"go.yaml.in/yaml/v2"

	testdata "github.com/grafana/alloy/internal/component/pyroscope/write/promhttp2/testdata"
)

const (
	TLSCAChainPath        = "testdata/tls-ca-chain.pem"
	ServerCertificatePath = "testdata/server.crt"
	ClientCertificatePath = "testdata/client.crt"
	WrongClientCertPath   = "testdata/self-signed-client.crt"
	EmptyFile             = "testdata/empty"
	MissingCA             = "missing/ca.crt"
	MissingCert           = "missing/cert.crt"
	MissingKey            = "missing/secret.key"

	ExpectedMessage                   = "I'm here to serve you!!!"
	ExpectedError                     = "expected error"
	AuthorizationCredentials          = "theanswertothegreatquestionoflifetheuniverseandeverythingisfortytwo"
	AuthorizationCredentialsFile      = "testdata/bearer.token"
	AuthorizationType                 = "APIKEY"
	BearerToken                       = AuthorizationCredentials
	BearerTokenFile                   = AuthorizationCredentialsFile
	MissingBearerTokenFile            = "missing/bearer.token"
	ExpectedBearer                    = "Bearer " + BearerToken
	ExpectedAuthenticationCredentials = AuthorizationType + " " + BearerToken
	ExpectedUsername                  = "arthurdent"
	ExpectedPassword                  = "42"
	ExpectedAccessToken               = "12345"
)

var invalidHTTPClientConfigs = []struct {
	httpClientConfigFile string
	errMsg               string
}{
	{
		httpClientConfigFile: "testdata/http.conf.bearer-token-and-file-set.bad.yml",
		errMsg:               "at most one of bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.empty.bad.yml",
		errMsg:               "at most one of basic_auth, oauth2, bearer_token & bearer_token_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth.too-much.bad.yaml",
		errMsg:               "at most one of basic_auth password, password_file & password_ref must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth.bad-username.yaml",
		errMsg:               "at most one of basic_auth username, username_file & username_ref must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.mix-bearer-and-creds.bad.yaml",
		errMsg:               "authorization is not compatible with bearer_token & bearer_token_file",
	},
	{
		httpClientConfigFile: "testdata/http.conf.auth-creds-and-file-set.too-much.bad.yaml",
		errMsg:               "at most one of authorization credentials & credentials_file must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth-and-auth-creds.too-much.bad.yaml",
		errMsg:               "at most one of basic_auth, oauth2 & authorization must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.basic-auth-and-oauth2.too-much.bad.yaml",
		errMsg:               "at most one of basic_auth, oauth2 & authorization must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.auth-creds-no-basic.bad.yaml",
		errMsg:               `authorization type cannot be set to "basic", use "basic_auth" instead`,
	},
	{
		httpClientConfigFile: "testdata/http.conf.oauth2-secret-and-file-set.bad.yml",
		errMsg:               "at most one of oauth2 client_secret, client_secret_file & client_secret_ref must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.oauth2-certificate-and-file-set.bad.yml",
		errMsg:               "at most one of oauth2 client_certificate_key, client_certificate_key_file & client_certificate_key_ref must be configured using grant-type=urn:ietf:params:oauth:grant-type:jwt-beare",
	},
	{
		httpClientConfigFile: "testdata/http.conf.oauth2-no-client-id.bad.yaml",
		errMsg:               "oauth2 client_id must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.oauth2-no-token-url.bad.yaml",
		errMsg:               "oauth2 token_url must be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.proxy-from-env.bad.yaml",
		errMsg:               "if proxy_from_environment is configured, proxy_url must not be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.no-proxy.bad.yaml",
		errMsg:               "if proxy_from_environment is configured, no_proxy must not be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.no-proxy-without-proxy-url.bad.yaml",
		errMsg:               "if no_proxy is configured, proxy_url must also be configured",
	},
	{
		httpClientConfigFile: "testdata/http.conf.headers-reserved.bad.yaml",
		errMsg:               `setting header "User-Agent" is not allowed`,
	},
}

func newTestServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, error) {
	t.Helper()
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(handler))
	serverKeyPath := testdata.ServerKeyPath(t)

	tlsCAChain, err := os.ReadFile(TLSCAChainPath)
	if err != nil {
		return nil, fmt.Errorf("Can't read %s", TLSCAChainPath)
	}
	serverCertificate, err := tls.LoadX509KeyPair(ServerCertificatePath, serverKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Can't load X509 key pair %s - %s", ServerCertificatePath, serverKeyPath)
	}

	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	testServer.TLS = &tls.Config{
		Certificates: make([]tls.Certificate, 1),
		RootCAs:      rootCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    rootCAs,
	}
	testServer.TLS.Certificates[0] = serverCertificate

	testServer.StartTLS()

	return testServer, nil
}

func TestNewClientFromConfig(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	newClientValidConfig := []struct {
		clientConfig commonconfig.HTTPClientConfig
		handler      func(w http.ResponseWriter, r *http.Request)
	}{
		{
			clientConfig: commonconfig.HTTPClientConfig{
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             "",
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: true,
				},
			},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, ExpectedMessage)
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				BearerToken: BearerToken,
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				BearerTokenFile: BearerTokenFile,
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				Authorization: &commonconfig.Authorization{Credentials: BearerToken},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				Authorization: &commonconfig.Authorization{CredentialsFile: AuthorizationCredentialsFile, Type: AuthorizationType},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedAuthenticationCredentials {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedAuthenticationCredentials, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				Authorization: &commonconfig.Authorization{
					Type: AuthorizationType,
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if strings.TrimSpace(bearer) != AuthorizationType {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						AuthorizationType, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				Authorization: &commonconfig.Authorization{
					Credentials: AuthorizationCredentials,
					Type:        AuthorizationType,
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedAuthenticationCredentials {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedAuthenticationCredentials, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				Authorization: &commonconfig.Authorization{
					CredentialsFile: BearerTokenFile,
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				bearer := r.Header.Get("Authorization")
				if bearer != ExpectedBearer {
					fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
						ExpectedBearer, bearer)
				} else {
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				BasicAuth: &commonconfig.BasicAuth{
					Username: ExpectedUsername,
					Password: ExpectedPassword,
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				switch {
				case !ok:
					fmt.Fprintf(w, "The Authorization header wasn't set")
				case ExpectedUsername != username:
					fmt.Fprintf(w, "The expected username (%s) differs from the obtained username (%s).", ExpectedUsername, username)
				case ExpectedPassword != password:
					fmt.Fprintf(w, "The expected password (%s) differs from the obtained password (%s).", ExpectedPassword, password)
				default:
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				FollowRedirects: true,
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/redirected":
					fmt.Fprint(w, ExpectedMessage)
				default:
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
					fmt.Fprint(w, "It should follow the redirect.")
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				FollowRedirects: false,
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/redirected":
					fmt.Fprint(w, "The redirection was followed.")
				default:
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
					fmt.Fprint(w, ExpectedMessage)
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				OAuth2: &commonconfig.OAuth2{
					ClientID: "ExpectedUsername",
					TLSConfig: commonconfig.TLSConfig{
						CAFile:             TLSCAChainPath,
						CertFile:           ClientCertificatePath,
						KeyFile:            clientKeyNoPassPath,
						ServerName:         "",
						InsecureSkipVerify: false,
					},
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					res, _ := json.Marshal(oauth2TestServerResponse{
						AccessToken: ExpectedAccessToken,
						TokenType:   "Bearer",
					})
					w.Header().Add("Content-Type", "application/json")
					_, _ = w.Write(res)

				default:
					authorization := r.Header.Get("Authorization")
					if authorization != "Bearer "+ExpectedAccessToken {
						fmt.Fprintf(w, "Expected Authorization header %q, got %q", "Bearer "+ExpectedAccessToken, authorization)
					} else {
						fmt.Fprint(w, ExpectedMessage)
					}
				}
			},
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				OAuth2: &commonconfig.OAuth2{
					ClientID:     "ExpectedUsername",
					ClientSecret: "ExpectedPassword",
					TLSConfig: commonconfig.TLSConfig{
						CAFile:             TLSCAChainPath,
						CertFile:           ClientCertificatePath,
						KeyFile:            clientKeyNoPassPath,
						ServerName:         "",
						InsecureSkipVerify: false,
					},
				},
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             TLSCAChainPath,
					CertFile:           ClientCertificatePath,
					KeyFile:            clientKeyNoPassPath,
					ServerName:         "",
					InsecureSkipVerify: false,
				},
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/token":
					res, _ := json.Marshal(oauth2TestServerResponse{
						AccessToken: ExpectedAccessToken,
						TokenType:   "Bearer",
					})
					w.Header().Add("Content-Type", "application/json")
					_, _ = w.Write(res)

				default:
					authorization := r.Header.Get("Authorization")
					if authorization != "Bearer "+ExpectedAccessToken {
						fmt.Fprintf(w, "Expected Authorization header %q, got %q", "Bearer "+ExpectedAccessToken, authorization)
					} else {
						fmt.Fprint(w, ExpectedMessage)
					}
				}
			},
		},
	}

	for _, validConfig := range newClientValidConfig {
		t.Run("", func(t *testing.T) {
			testServer, err := newTestServer(t, validConfig.handler)
			require.NoError(t, err)
			defer testServer.Close()

			if validConfig.clientConfig.OAuth2 != nil {
				// We don't have access to the test server's URL when configuring the test cases,
				// so it has to be specified here.
				validConfig.clientConfig.OAuth2.TokenURL = testServer.URL + "/token"
			}

			err = validConfig.clientConfig.Validate()
			require.NoError(t, err)
			client, err := NewClientFromConfig(validConfig.clientConfig, "test")
			if err != nil {
				t.Errorf("Can't create a client from this config: %+v", validConfig.clientConfig)
				return
			}

			response, err := client.Get(testServer.URL)
			if err != nil {
				t.Errorf("Can't connect to the test server using this config: %+v: %v", validConfig.clientConfig, err)
				return
			}

			message, err := io.ReadAll(response.Body)
			response.Body.Close()
			if err != nil {
				t.Errorf("Can't read the server response body using this config: %+v", validConfig.clientConfig)
				return
			}

			trimMessage := strings.TrimSpace(string(message))
			if ExpectedMessage != trimMessage {
				t.Errorf("The expected message (%s) differs from the obtained message (%s) using this config: %+v",
					ExpectedMessage, trimMessage, validConfig.clientConfig)
			}
		})
	}
}

func TestProxyConfiguration(t *testing.T) {
	testcases := map[string]struct {
		testFn  string
		loader  func(string) (*commonconfig.HTTPClientConfig, []byte, error)
		isValid bool
	}{
		"good yaml": {
			testFn:  "testdata/http.conf.proxy-headers.good.yml",
			loader:  commonconfig.LoadHTTPConfigFile,
			isValid: true,
		},
		"bad yaml": {
			testFn:  "testdata/http.conf.proxy-headers.bad.yml",
			loader:  commonconfig.LoadHTTPConfigFile,
			isValid: false,
		},
		"good json": {
			testFn:  "testdata/http.conf.proxy-headers.good.json",
			loader:  loadHTTPConfigJSONFile,
			isValid: true,
		},
		"bad json": {
			testFn:  "testdata/http.conf.proxy-headers.bad.json",
			loader:  loadHTTPConfigJSONFile,
			isValid: false,
		},
	}

	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			_, _, err := tc.loader(tc.testFn)
			if tc.isValid {
				require.NoErrorf(t, err, "Error validating %s: %s", tc.testFn, err)
			} else {
				require.Errorf(t, err, "Expecting error validating %s but got %s", tc.testFn, err)
			}
		})
	}
}

func TestNewClientFromInvalidConfig(t *testing.T) {
	invalidCA := testdata.InvalidCA(t)

	newClientInvalidConfig := []struct {
		clientConfig commonconfig.HTTPClientConfig
		errorMsg     string
	}{
		{
			clientConfig: commonconfig.HTTPClientConfig{
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             MissingCA,
					InsecureSkipVerify: true,
				},
			},
			errorMsg: "unable to read CA cert: unable to read file " + MissingCA,
		},
		{
			clientConfig: commonconfig.HTTPClientConfig{
				TLSConfig: commonconfig.TLSConfig{
					CAFile:             invalidCA,
					InsecureSkipVerify: true,
				},
			},
			errorMsg: "unable to use specified CA cert file",
		},
	}

	for _, invalidConfig := range newClientInvalidConfig {
		client, err := NewClientFromConfig(invalidConfig.clientConfig, "test")
		if client != nil {
			t.Errorf("A client instance was returned instead of nil using this config: %+v", invalidConfig.clientConfig)
		}
		if err == nil {
			t.Errorf("No error was returned using this config: %+v", invalidConfig.clientConfig)
		}
		if !strings.Contains(err.Error(), invalidConfig.errorMsg) {
			t.Errorf("Expected error %q does not contain %q", err.Error(), invalidConfig.errorMsg)
		}
	}
}

func TestCustomDialContextFunc(t *testing.T) {
	dialFn := func(_ context.Context, _, _ string) (net.Conn, error) {
		return nil, errors.New(ExpectedError)
	}

	cfg := commonconfig.HTTPClientConfig{}
	client, err := NewClientFromConfig(cfg, "test", WithDialContextFunc(dialFn))
	require.NoErrorf(t, err, "Can't create a client from this config: %+v", cfg)

	_, err = client.Get("http://localhost")
	if err == nil || !strings.Contains(err.Error(), ExpectedError) {
		t.Errorf("Expected error %q but got %q", ExpectedError, err)
	}
}

func TestCustomIdleConnTimeout(t *testing.T) {
	timeout := time.Second * 5

	cfg := commonconfig.HTTPClientConfig{}
	rt, err := NewRoundTripperFromConfig(cfg, "test", WithIdleConnTimeout(timeout))
	require.NoErrorf(t, err, "Can't create a round-tripper from this config: %+v", cfg)

	transport, ok := rt.(*http.Transport)
	require.Truef(t, ok, "Unexpected transport: %+v", transport)

	require.Equalf(t, transport.IdleConnTimeout, timeout, "Unexpected idle connection timeout: %+v", timeout)
}

func TestMissingBearerAuthFile(t *testing.T) {
	cfg := commonconfig.HTTPClientConfig{
		BearerTokenFile: MissingBearerTokenFile,
		TLSConfig: commonconfig.TLSConfig{
			CAFile:             TLSCAChainPath,
			CertFile:           ClientCertificatePath,
			KeyFile:            testdata.ClientKeyNoPassPath(t),
			ServerName:         "",
			InsecureSkipVerify: false,
		},
	}
	handler := func(w http.ResponseWriter, r *http.Request) {
		bearer := r.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			fmt.Fprintf(w, "The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		} else {
			fmt.Fprint(w, ExpectedMessage)
		}
	}

	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	client, err := NewClientFromConfig(cfg, "test")
	require.NoError(t, err)

	_, err = client.Get(testServer.URL)
	require.Errorf(t, err, "No error is returned here")

	require.ErrorContainsf(t, err, "unable to read authorization credentials: unable to read file missing/bearer.token: open missing/bearer.token: no such file or directory", "wrong error message being returned")
}

func TestBearerAuthRoundTripper(t *testing.T) {
	const (
		newBearerToken = "goodbyeandthankyouforthefish"
	)

	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", commonconfig.NewInlineSecret(BearerToken), fakeRoundTripper)
	request, _ := http.NewRequest(http.MethodGet, "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	_, err := bearerAuthRoundTripper.RoundTrip(request)
	if err != nil {
		t.Errorf("unexpected error while executing RoundTrip: %s", err.Error())
	}

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", commonconfig.NewInlineSecret(newBearerToken), fakeRoundTripper)
	request, _ = http.NewRequest(http.MethodGet, "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	_, err = bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
	if err != nil {
		t.Errorf("unexpected error while executing RoundTrip: %s", err.Error())
	}
}

func TestBearerAuthFileRoundTripper(t *testing.T) {
	fakeRoundTripper := NewRoundTripCheckRequest(func(req *http.Request) {
		bearer := req.Header.Get("Authorization")
		if bearer != ExpectedBearer {
			t.Errorf("The expected Bearer Authorization (%s) differs from the obtained Bearer Authorization (%s)",
				ExpectedBearer, bearer)
		}
	}, nil, nil)

	// Normal flow.
	bearerAuthRoundTripper := commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", commonconfig.NewFileSecret(BearerTokenFile), fakeRoundTripper)
	request, _ := http.NewRequest(http.MethodGet, "/hitchhiker", nil)
	request.Header.Set("User-Agent", "Douglas Adams mind")
	_, err := bearerAuthRoundTripper.RoundTrip(request)
	if err != nil {
		t.Errorf("unexpected error while executing RoundTrip: %s", err.Error())
	}

	// Should honor already Authorization header set.
	bearerAuthRoundTripperShouldNotModifyExistingAuthorization := commonconfig.NewAuthorizationCredentialsRoundTripper("Bearer", commonconfig.NewFileSecret(MissingBearerTokenFile), fakeRoundTripper)
	request, _ = http.NewRequest(http.MethodGet, "/hitchhiker", nil)
	request.Header.Set("Authorization", ExpectedBearer)
	_, err = bearerAuthRoundTripperShouldNotModifyExistingAuthorization.RoundTrip(request)
	if err != nil {
		t.Errorf("unexpected error while executing RoundTrip: %s", err.Error())
	}
}

func TestTLSConfig(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	configTLSConfig := commonconfig.TLSConfig{
		CAFile:             TLSCAChainPath,
		CertFile:           ClientCertificatePath,
		KeyFile:            clientKeyNoPassPath,
		ServerName:         "localhost",
		InsecureSkipVerify: false,
	}

	tlsCAChain, err := os.ReadFile(TLSCAChainPath)
	require.NoErrorf(t, err, "Can't read the CA certificate chain (%s)",
		TLSCAChainPath)
	rootCAs := x509.NewCertPool()
	rootCAs.AppendCertsFromPEM(tlsCAChain)

	expectedTLSConfig := &tls.Config{
		RootCAs:            rootCAs,
		ServerName:         configTLSConfig.ServerName,
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify,
	}

	tlsConfig, err := commonconfig.NewTLSConfig(&configTLSConfig)
	require.NoErrorf(t, err, "Can't create a new TLS Config from a configuration (%s).", err)

	clientCertificate, err := tls.LoadX509KeyPair(ClientCertificatePath, clientKeyNoPassPath)
	require.NoErrorf(t, err, "Can't load the client key pair ('%s' and '%s'). Reason: %s",
		ClientCertificatePath, clientKeyNoPassPath, err)
	cert, err := tlsConfig.GetClientCertificate(nil)
	require.NoErrorf(t, err, "unexpected error returned by tlsConfig.GetClientCertificate(): %s", err)
	require.Truef(t, reflect.DeepEqual(cert, &clientCertificate), "Unexpected client certificate result: \n\n%+v\n expected\n\n%+v", cert, clientCertificate)

	// tlsConfig.rootCAs.LazyCerts contains functions getCert() in go 1.16, which are
	// never equal. Compare the Subjects instead.
	//nolint:staticcheck // Ignore SA1019. (*CertPool).Subjects is deprecated because it may not include the system certs but it isn't the case here.
	require.Truef(t, reflect.DeepEqual(tlsConfig.RootCAs.Subjects(), expectedTLSConfig.RootCAs.Subjects()), "Unexpected RootCAs result: \n\n%+v\n expected\n\n%+v", tlsConfig.RootCAs.Subjects(), expectedTLSConfig.RootCAs.Subjects())
	tlsConfig.RootCAs = nil
	expectedTLSConfig.RootCAs = nil

	// Non-nil functions are never equal.
	tlsConfig.GetClientCertificate = nil

	require.Truef(t, reflect.DeepEqual(tlsConfig, expectedTLSConfig), "Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
}

func TestTLSConfigEmpty(t *testing.T) {
	configTLSConfig := commonconfig.TLSConfig{
		InsecureSkipVerify: true,
	}

	expectedTLSConfig := &tls.Config{
		InsecureSkipVerify: configTLSConfig.InsecureSkipVerify,
	}

	tlsConfig, err := commonconfig.NewTLSConfig(&configTLSConfig)
	require.NoErrorf(t, err, "Can't create a new TLS Config from a configuration (%s).", err)

	require.Truef(t, reflect.DeepEqual(tlsConfig, expectedTLSConfig), "Unexpected TLS Config result: \n\n%+v\n expected\n\n%+v", tlsConfig, expectedTLSConfig)
}

func TestTLSConfigInvalidCA(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	invalidTLSConfig := []struct {
		configTLSConfig commonconfig.TLSConfig
		errorMessage    string
	}{
		{
			configTLSConfig: commonconfig.TLSConfig{
				CAFile:             MissingCA,
				CertFile:           "",
				KeyFile:            "",
				ServerName:         "",
				InsecureSkipVerify: false,
			},
			errorMessage: "unable to read CA cert: unable to read file " + MissingCA,
		},
		{
			configTLSConfig: commonconfig.TLSConfig{
				CAFile:             "",
				CertFile:           MissingCert,
				KeyFile:            clientKeyNoPassPath,
				ServerName:         "",
				InsecureSkipVerify: false,
			},
			errorMessage: "unable to read specified client cert: unable to read file " + MissingCert,
		},
		{
			configTLSConfig: commonconfig.TLSConfig{
				CAFile:             "",
				CertFile:           ClientCertificatePath,
				KeyFile:            MissingKey,
				ServerName:         "",
				InsecureSkipVerify: false,
			},
			errorMessage: "unable to read specified client key: unable to read file " + MissingKey,
		},
		{
			configTLSConfig: commonconfig.TLSConfig{
				CAFile:             "",
				Cert:               readFile(t, ClientCertificatePath),
				CertFile:           ClientCertificatePath,
				KeyFile:            clientKeyNoPassPath,
				ServerName:         "",
				InsecureSkipVerify: false,
			},
			errorMessage: "at most one of cert, cert_file & cert_ref must be configured",
		},
		{
			configTLSConfig: commonconfig.TLSConfig{
				CAFile:             "",
				CertFile:           ClientCertificatePath,
				Key:                commonconfig.Secret(readFile(t, clientKeyNoPassPath)),
				KeyFile:            clientKeyNoPassPath,
				ServerName:         "",
				InsecureSkipVerify: false,
			},
			errorMessage: "at most one of key and key_file must be configured",
		},
	}

	for _, anInvalididTLSConfig := range invalidTLSConfig {
		tlsConfig, err := commonconfig.NewTLSConfig(&anInvalididTLSConfig.configTLSConfig)
		if tlsConfig != nil && err == nil {
			t.Errorf("The TLS Config could be created even with this %+v", anInvalididTLSConfig.configTLSConfig)
			continue
		}
		if !strings.Contains(err.Error(), anInvalididTLSConfig.errorMessage) {
			t.Errorf("The expected error should contain %s, but got %s", anInvalididTLSConfig.errorMessage, err)
		}
	}
}

func TestBasicAuthNoPassword(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.no-password.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	username, _ := rt.username.Fetch(context.Background())
	require.Equalf(t, "user", username, "Bad HTTP client username: %s", username)
	require.Nilf(t, rt.password, "Expected empty HTTP client password")
}

func TestBasicAuthNoUsername(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.no-username.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	if rt.username != nil {
		t.Errorf("Got unexpected username")
	}
	if password, _ := rt.password.Fetch(context.Background()); password != "secret" {
		t.Errorf("Unexpected HTTP client password: %s", password)
	}
}

func TestBasicAuthPasswordFile(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.good.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	if username, _ := rt.username.Fetch(context.Background()); username != "user" {
		t.Errorf("Bad HTTP client username: %s", username)
	}
	if password, _ := rt.password.Fetch(context.Background()); password != "foobar" {
		t.Errorf("Bad HTTP client password: %s", password)
	}
}

// basicAuthRT wraps a commonconfig.basicAuthRoundTripper (unexported) using
// reflection, so tests can inspect the username/password fields.
// Changed compared to prometheus tests due to use of unexported types.
type basicAuthRT struct {
	username commonconfig.SecretReader
	password commonconfig.SecretReader
}

func assertBasicAuthRoundTripper(t *testing.T, transport http.RoundTripper) basicAuthRT {
	t.Helper()
	v := reflect.ValueOf(transport)
	require.Equal(t, reflect.Pointer, v.Kind(), "expected pointer, got %s", v.Kind())
	require.Equal(t, "basicAuthRoundTripper", v.Elem().Type().Name(),
		"expected basicAuthRoundTripper, got %s", v.Elem().Type().Name())
	elem := v.Elem()
	var result basicAuthRT
	// Fields are unexported, so we use unsafe to read them.
	u := elem.FieldByName("username")
	require.True(t, u.IsValid(), "basicAuthRoundTripper missing username field")
	if !u.IsNil() {
		result.username = reflect.NewAt(u.Type(), u.Addr().UnsafePointer()).Elem().Interface().(commonconfig.SecretReader)
	}
	p := elem.FieldByName("password")
	require.True(t, p.IsValid(), "basicAuthRoundTripper missing password field")
	if !p.IsNil() {
		result.password = reflect.NewAt(p.Type(), p.Addr().UnsafePointer()).Elem().Interface().(commonconfig.SecretReader)
	}
	return result
}

type secretManager struct {
	data map[string]string
}

func (m *secretManager) Fetch(_ context.Context, secretRef string) (string, error) {
	secretData, ok := m.data[secretRef]
	if !ok {
		return "", fmt.Errorf("unknown secret %s", secretRef)
	}
	return secretData, nil
}

func TestBasicAuthSecretManager(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.ref.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	manager := secretManager{
		data: map[string]string{
			"admin": "user",
			"pass":  "foobar",
		},
	}
	client, err := NewClientFromConfig(*cfg, "test", WithSecretManager(&manager))
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	if username, _ := rt.username.Fetch(context.Background()); username != "user" {
		t.Errorf("Bad HTTP client username: %s", username)
	}
	if password, _ := rt.password.Fetch(context.Background()); password != "foobar" {
		t.Errorf("Bad HTTP client password: %s", password)
	}
}

func TestBasicAuthSecretManagerNotFound(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.ref.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	manager := secretManager{
		data: map[string]string{
			"admin1": "user",
			"foobar": "pass",
		},
	}
	client, err := NewClientFromConfig(*cfg, "test", WithSecretManager(&manager))
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	if _, err := rt.username.Fetch(context.Background()); !strings.Contains(err.Error(), "unknown secret admin") {
		t.Errorf("Unexpected error message: %s", err)
	}
	if _, err := rt.password.Fetch(context.Background()); !strings.Contains(err.Error(), "unknown secret pass") {
		t.Errorf("Unexpected error message: %s", err)
	}
}

func TestBasicUsernameFile(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.basic-auth.username-file.good.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	rt := assertBasicAuthRoundTripper(t, client.Transport)

	if username, _ := rt.username.Fetch(context.Background()); username != "testuser" {
		t.Errorf("Bad HTTP client username: %s", username)
	}
	if password, _ := rt.password.Fetch(context.Background()); password != "foobar" {
		t.Errorf("Bad HTTP client passwordFile: %s", password)
	}
}

func writeCertificate(src, dst string) {
	b, err := os.ReadFile(src)
	if err != nil {
		panic(fmt.Sprintf("Couldn't read %q: %v", src, err))
	}
	if err := os.WriteFile(dst, b, 0o664); err != nil {
		panic(err)
	}
}

func TestTLSRoundTripper(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	tmpDir := t.TempDir()

	ca, cert, key := filepath.Join(tmpDir, "ca"), filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	testCases := []struct {
		ca   string
		cert string
		key  string

		errMsg string
	}{
		{
			// Valid certs.
			ca:   TLSCAChainPath,
			cert: ClientCertificatePath,
			key:  clientKeyNoPassPath,
		},
		{
			// CA not matching.
			ca:   ClientCertificatePath,
			cert: ClientCertificatePath,
			key:  clientKeyNoPassPath,

			errMsg: "certificate signed by unknown authority",
		},
		{
			// Invalid client cert+key.
			ca:   TLSCAChainPath,
			cert: WrongClientCertPath,
			key:  testdata.WrongClientKeyPath(t),

			errMsg: "remote error: tls",
		},
		{
			// CA file empty
			ca:   EmptyFile,
			cert: ClientCertificatePath,
			key:  clientKeyNoPassPath,

			errMsg: "unable to use specified CA cert",
		},
		{
			// cert file empty
			ca:   TLSCAChainPath,
			cert: EmptyFile,
			key:  clientKeyNoPassPath,

			errMsg: "failed to find any PEM data in certificate input",
		},
		{
			// key file empty
			ca:   TLSCAChainPath,
			cert: ClientCertificatePath,
			key:  EmptyFile,

			errMsg: "failed to find any PEM data in key input",
		},
		{
			// Valid certs again.
			ca:   TLSCAChainPath,
			cert: ClientCertificatePath,
			key:  clientKeyNoPassPath,
		},
	}

	cfg := commonconfig.HTTPClientConfig{
		TLSConfig: commonconfig.TLSConfig{
			CAFile:             ca,
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: false,
		},
	}

	var c *http.Client
	for i, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			writeCertificate(tc.ca, ca)
			writeCertificate(tc.cert, cert)
			writeCertificate(tc.key, key)
			if c == nil {
				c, err = NewClientFromConfig(cfg, "test")
				require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
			require.NoErrorf(t, err, "Error creating HTTP request: %v", err)
			r, err := c.Do(req)
			if len(tc.errMsg) > 0 {
				if err == nil {
					r.Body.Close()
					t.Fatalf("Could connect to the test server.")
				}
				require.ErrorContainsf(t, err, tc.errMsg, "Expected error message to contain %q, got %q", tc.errMsg, err)
				return
			}

			require.NoErrorf(t, err, "Can't connect to the test server")

			b, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				t.Errorf("Can't read the server response body")
			}

			got := strings.TrimSpace(string(b))
			if ExpectedMessage != got {
				t.Errorf("The expected message %q differs from the obtained message %q", ExpectedMessage, got)
			}
		})
	}
}

func TestTLSRoundTripper_Inline(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	handler := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	testCases := []struct {
		caText, caFile     string
		certText, certFile string
		keyText, keyFile   string

		errMsg string
	}{
		{
			// File-based everything.
			caFile:   TLSCAChainPath,
			certFile: ClientCertificatePath,
			keyFile:  clientKeyNoPassPath,
		},
		{
			// Inline CA.
			caText:   readFile(t, TLSCAChainPath),
			certFile: ClientCertificatePath,
			keyFile:  clientKeyNoPassPath,
		},
		{
			// Inline cert.
			caFile:   TLSCAChainPath,
			certText: readFile(t, ClientCertificatePath),
			keyFile:  clientKeyNoPassPath,
		},
		{
			// Inline key.
			caFile:   TLSCAChainPath,
			certFile: ClientCertificatePath,
			keyText:  readFile(t, clientKeyNoPassPath),
		},
		{
			// Inline everything.
			caText:   readFile(t, TLSCAChainPath),
			certText: readFile(t, ClientCertificatePath),
			keyText:  readFile(t, clientKeyNoPassPath),
		},

		{
			// Invalid inline CA.
			caText:   "badca",
			certText: readFile(t, ClientCertificatePath),
			keyText:  readFile(t, clientKeyNoPassPath),

			errMsg: "unable to use specified CA cert inline",
		},
		{
			// Invalid cert.
			caText:   readFile(t, TLSCAChainPath),
			certText: "badcert",
			keyText:  readFile(t, clientKeyNoPassPath),

			errMsg: "failed to find any PEM data in certificate input",
		},
		{
			// Invalid key.
			caText:   readFile(t, TLSCAChainPath),
			certText: readFile(t, ClientCertificatePath),
			keyText:  "badkey",

			errMsg: "failed to find any PEM data in key input",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			cfg := commonconfig.HTTPClientConfig{
				TLSConfig: commonconfig.TLSConfig{
					CA:                 tc.caText,
					CAFile:             tc.caFile,
					Cert:               tc.certText,
					CertFile:           tc.certFile,
					Key:                commonconfig.Secret(tc.keyText),
					KeyFile:            tc.keyFile,
					InsecureSkipVerify: false,
				},
			}

			c, err := NewClientFromConfig(cfg, "test")
			if tc.errMsg != "" {
				require.ErrorContainsf(t, err, tc.errMsg, "Expected error message to contain %q, got %q", tc.errMsg, err)
				return
			} else if err != nil {
				t.Fatalf("Error creating HTTP Client: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
			require.NoErrorf(t, err, "Error creating HTTP request: %v", err)
			r, err := c.Do(req)
			require.NoErrorf(t, err, "Can't connect to the test server")

			b, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				t.Errorf("Can't read the server response body")
			}

			got := strings.TrimSpace(string(b))
			if ExpectedMessage != got {
				t.Errorf("The expected message %q differs from the obtained message %q", ExpectedMessage, got)
			}
		})
	}
}

func TestTLSRoundTripperRaces(t *testing.T) {
	tmpDir := t.TempDir()

	ca, cert, key := filepath.Join(tmpDir, "ca"), filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	cfg := commonconfig.HTTPClientConfig{
		TLSConfig: commonconfig.TLSConfig{
			CAFile:             ca,
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: false,
		},
	}

	var c *http.Client
	writeCertificate(TLSCAChainPath, ca)
	writeCertificate(ClientCertificatePath, cert)
	writeCertificate(testdata.ClientKeyNoPassPath(t), key)
	c, err = NewClientFromConfig(cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	var wg sync.WaitGroup
	ch := make(chan struct{})
	var total, ok uberatomic.Int64
	// Spawn 10 Go routines polling the server concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ch:
					return
				default:
					total.Inc()
					r, err := c.Get(testServer.URL)
					if err == nil {
						r.Body.Close()
						ok.Inc()
					}
				}
			}
		}()
	}

	// Change the CA file every 10ms for 1 second.
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			tick := time.NewTicker(10 * time.Millisecond)
			<-tick.C
			if i%2 == 0 {
				writeCertificate(ClientCertificatePath, ca)
			} else {
				writeCertificate(TLSCAChainPath, ca)
			}
			i++
			if i > 100 {
				close(ch)
				return
			}
		}
	}()

	wg.Wait()
	require.NotEqualf(t, ok, total, "Expecting some requests to fail but got %d/%d successful requests", ok, total)
}

func TestHideHTTPClientConfigSecrets(t *testing.T) {
	c, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.good.yml")
	require.NoErrorf(t, err, "Error parsing %s: %s", "testdata/http.conf.good.yml", err)

	// String method must not reveal authentication credentials.
	s := c.String()
	require.NotContainsf(t, s, "mysecret", "http client config's String method reveals authentication credentials.")
}

func TestDefaultFollowRedirect(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	if !cfg.FollowRedirects {
		t.Errorf("follow_redirects should be true")
	}
}

func TestValidateHTTPConfig(t *testing.T) {
	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.good.yml")
	if err != nil {
		t.Errorf("Error loading HTTP client config: %v", err)
	}
	err = cfg.Validate()
	require.NoErrorf(t, err, "Error validating %s: %s", "testdata/http.conf.good.yml", err)
}

func TestInvalidHTTPConfigs(t *testing.T) {
	for _, ee := range invalidHTTPClientConfigs {
		_, _, err := commonconfig.LoadHTTPConfigFile(ee.httpClientConfigFile)
		if err == nil {
			t.Errorf("Expected error with config %q but got none", ee.httpClientConfigFile)
			continue
		}
		if !strings.Contains(err.Error(), ee.errMsg) {
			t.Errorf("Expected error for invalid HTTP client configuration to contain %q but got: %s", ee.errMsg, err)
		}
	}
}

type roundTrip struct {
	theResponse *http.Response
	theError    error
}

func (rt *roundTrip) RoundTrip(*http.Request) (*http.Response, error) {
	return rt.theResponse, rt.theError
}

type roundTripCheckRequest struct {
	checkRequest func(*http.Request)
	roundTrip
}

func (rt *roundTripCheckRequest) RoundTrip(r *http.Request) (*http.Response, error) {
	rt.checkRequest(r)
	return rt.theResponse, rt.theError
}

// NewRoundTripCheckRequest creates a new instance of a type that implements http.RoundTripper,
// which before returning theResponse and theError, executes checkRequest against a http.Request.
func NewRoundTripCheckRequest(checkRequest func(*http.Request), theResponse *http.Response, theError error) http.RoundTripper {
	return &roundTripCheckRequest{
		checkRequest: checkRequest,
		roundTrip: roundTrip{
			theResponse: theResponse,
			theError:    theError,
		},
	}
}

type oauth2TestServerResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type testOAuthServer struct {
	tokenTS *httptest.Server
	ts      *httptest.Server
}

// newTestOAuthServer returns a new test server with the expected base64 encoded client ID and secret.
func newTestOAuthServer(t testing.TB, expectedAuth func(testing.TB, string)) testOAuthServer {
	var previousAuth string
	tokenTS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			require.NoErrorf(t, r.ParseForm(), "Failed to parse form")
			auth = r.FormValue("assertion")
		}

		expectedAuth(t, auth)

		require.NotEqualf(t, auth, previousAuth, "token endpoint called twice")
		previousAuth = auth
		res, _ := json.Marshal(oauth2TestServerResponse{
			AccessToken: "12345",
			TokenType:   "Bearer",
		})
		w.Header().Add("Content-Type", "application/json")
		_, _ = w.Write(res)
	}))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		require.Equalf(t, "Bearer 12345", auth, "bad auth, expected %s, got %s", "Bearer 12345", auth)
		fmt.Fprintln(w, "Hello, client")
	}))
	return testOAuthServer{
		tokenTS: tokenTS,
		ts:      ts,
	}
}

func (s *testOAuthServer) url() string {
	return s.ts.URL
}

func (s *testOAuthServer) tokenURL() string {
	return s.tokenTS.URL
}

func (s *testOAuthServer) close() {
	s.tokenTS.Close()
	s.ts.Close()
}

func TestOAuth2(t *testing.T) {
	expectedAuth := new(string)
	ts := newTestOAuthServer(t, func(t testing.TB, auth string) {
		require.Equalf(t, *expectedAuth, auth, "bad auth, expected %s, got %s", *expectedAuth, auth)
	})
	defer ts.close()

	yamlConfig := fmt.Sprintf(`
client_id: 1
client_secret: 2
scopes:
 - A
 - B
token_url: %s
endpoint_params:
 hi: hello
`, ts.tokenURL())
	expectedConfig := commonconfig.OAuth2{
		ClientID:       "1",
		ClientSecret:   "2",
		Scopes:         []string{"A", "B"},
		EndpointParams: map[string]string{"hi": "hello"},
		TokenURL:       ts.tokenURL(),
	}

	var unmarshalledConfig commonconfig.OAuth2
	err := yaml.Unmarshal([]byte(yamlConfig), &unmarshalledConfig)
	require.NoErrorf(t, err, "Expected no error unmarshalling yaml, got %v", err)
	require.Truef(t, reflect.DeepEqual(unmarshalledConfig, expectedConfig), "Got unmarshalled config %v, expected %v", unmarshalledConfig, expectedConfig)

	secret := commonconfig.NewInlineSecret(string(expectedConfig.ClientSecret))
	rt, err := newOAuth2RoundTripper(secret, &expectedConfig, http.DefaultTransport, &defaultHTTPClientOptions)
	require.NoError(t, err)

	client := http.Client{
		Transport: rt,
	}

	// Default secret.
	*expectedAuth = "Basic MToy"
	resp, err := client.Get(ts.url())
	require.NoError(t, err)

	authorization := resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer', got '%s'", authorization)

	// Making a second request with the same secret should not re-call the token API.
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	// Empty secret.
	*expectedAuth = "Basic MTo="
	expectedConfig.ClientSecret = ""
	resp, err = client.Get(ts.url())
	require.NoError(t, err)

	authorization = resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer 12345', got '%s'", authorization)

	// Making a second request with the same secret should not re-call the token API.
	resp, err = client.Get(ts.url())
	require.NoError(t, err)

	// Update secret.
	*expectedAuth = "Basic MToxMjM0NTY3"
	expectedConfig.ClientSecret = "1234567"
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	// Making a second request with the same secret should not re-call the token API.
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	authorization = resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer 12345', got '%s'", authorization)
}

func TestOAuth2UserAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equalf(t, "myuseragent", r.Header.Get("User-Agent"), "Expected User-Agent header in oauth request to be 'myuseragent', got '%s'", r.Header.Get("User-Agent"))

		res, _ := json.Marshal(oauth2TestServerResponse{
			AccessToken: "12345",
			TokenType:   "Bearer",
		})
		w.Header().Add("Content-Type", "application/json")
		_, _ = w.Write(res)
	}))
	defer ts.Close()

	config := commonconfig.DefaultHTTPClientConfig
	config.OAuth2 = &commonconfig.OAuth2{
		ClientID:       "1",
		ClientSecret:   "2",
		Scopes:         []string{"A", "B"},
		EndpointParams: map[string]string{"hi": "hello"},
		TokenURL:       ts.URL + "/token",
	}

	rt, err := NewRoundTripperFromConfig(config, "test_oauth2", WithUserAgent("myuseragent"))
	require.NoError(t, err)

	client := http.Client{
		Transport: rt,
	}
	resp, err := client.Get(ts.URL)
	require.NoError(t, err)

	authorization := resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer 12345', got '%s'", authorization)
}

func TestHost(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equalf(t, "localhost.localdomain", r.Host, "Expected Host header in request to be 'localhost.localdomain', got '%s'", r.Host)

		w.Header().Add("Content-Type", "application/json")
	}))
	defer ts.Close()

	config := commonconfig.DefaultHTTPClientConfig

	rt, err := NewRoundTripperFromConfig(config, "test_host", WithHost("localhost.localdomain"))
	require.NoError(t, err)

	client := http.Client{
		Transport: rt,
	}
	_, err = client.Get(ts.URL)
	require.NoError(t, err)
}

func TestOAuth2WithFile(t *testing.T) {
	expectedAuth := new(string)
	ts := newTestOAuthServer(t, func(t testing.TB, auth string) {
		require.Equalf(t, *expectedAuth, auth, "bad auth, expected %s, got %s", *expectedAuth, auth)
	})
	defer ts.close()

	secretFile, err := os.CreateTemp(t.TempDir(), "oauth2_secret")
	require.NoError(t, err)

	yamlConfig := fmt.Sprintf(`
client_id: 1
client_secret_file: %s
scopes:
 - A
 - B
token_url: %s
endpoint_params:
 hi: hello
`, secretFile.Name(), ts.tokenURL())
	expectedConfig := commonconfig.OAuth2{
		ClientID:         "1",
		ClientSecretFile: secretFile.Name(),
		Scopes:           []string{"A", "B"},
		EndpointParams:   map[string]string{"hi": "hello"},
		TokenURL:         ts.tokenURL(),
	}

	var unmarshalledConfig commonconfig.OAuth2
	err = yaml.Unmarshal([]byte(yamlConfig), &unmarshalledConfig)
	require.NoErrorf(t, err, "Expected no error unmarshalling yaml, got %v", err)
	require.Truef(t, reflect.DeepEqual(unmarshalledConfig, expectedConfig), "Got unmarshalled config %v, expected %v", unmarshalledConfig, expectedConfig)

	secret := commonconfig.NewFileSecret(expectedConfig.ClientSecretFile)
	rt, err := newOAuth2RoundTripper(secret, &expectedConfig, http.DefaultTransport, &defaultHTTPClientOptions)
	require.NoError(t, err)

	client := http.Client{
		Transport: rt,
	}

	// Empty secret file.
	*expectedAuth = "Basic MTo="
	resp, err := client.Get(ts.url())
	require.NoError(t, err)

	authorization := resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer', got '%s'", authorization)

	// Making a second request with the same file content should not re-call the token API.
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	// File populated.
	*expectedAuth = "Basic MToxMjM0NTY="
	_, err = secretFile.Write([]byte("123456"))
	require.NoError(t, err)
	resp, err = client.Get(ts.url())
	require.NoError(t, err)

	authorization = resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer 12345', got '%s'", authorization)

	// Making a second request with the same file content should not re-call the token API.
	resp, err = client.Get(ts.url())
	require.NoError(t, err)

	// Update file.
	*expectedAuth = "Basic MToxMjM0NTY3"
	_, err = secretFile.Write([]byte("7"))
	require.NoError(t, err)
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	// Making a second request with the same file content should not re-call the token API.
	_, err = client.Get(ts.url())
	require.NoError(t, err)

	authorization = resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer 12345', got '%s'", authorization)
}

func TestOAuth2WithJWTAuth(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	ts := newTestOAuthServer(t, func(t testing.TB, auth string) {
		t.Helper()

		jwtParts := strings.Split(auth, ".")
		require.Lenf(t, jwtParts, 3, "Expected JWT to have 3 parts, got %d", len(jwtParts))

		// Decode the JWT payload.
		payload, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
		require.NoErrorf(t, err, "Failed to decode JWT payload: %v", err)

		var jwt struct {
			Aud     string `json:"aud"`
			Scope   string `json:"scope"`
			Sub     string `json:"sub"`
			Iss     string `json:"iss"`
			Integer int    `json:"integer"`
		}

		err = json.Unmarshal(payload, &jwt)
		require.NoErrorf(t, err, "Failed to unmarshal JWT payload: %v", err)

		require.Equalf(t, "common-test", jwt.Aud, "Expected aud to be 'common-test', got '%s'", jwt.Aud)
		require.Equalf(t, "A B", jwt.Scope, "Expected scope to be 'A B', got '%s'", jwt.Scope)
		require.Equalf(t, "common", jwt.Sub, "Expected sub to be 'common', got '%s'", jwt.Sub)
		require.Equalf(t, "https://example.com", jwt.Iss, "Expected iss to be 'https://example.com', got '%s'", jwt.Iss)
		require.Equalf(t, 1, jwt.Integer, "Expected integer to be 1, got '%d'", jwt.Integer)
	})
	defer ts.close()

	yamlConfig := fmt.Sprintf(`
grant_type: urn:ietf:params:oauth:grant-type:jwt-bearer
client_id: 1
client_certificate_key_file: %s
scopes:
 - A
 - B
claims:
  iss: "https://example.com"
  aud: common-test
  sub: common
  integer: 1
token_url: %s
endpoint_params:
  hi: hello
`, clientKeyNoPassPath, ts.tokenURL())
	expectedConfig := commonconfig.OAuth2{
		GrantType:                grantTypeJWTBearer,
		ClientID:                 "1",
		ClientCertificateKeyFile: clientKeyNoPassPath,
		Scopes:                   []string{"A", "B"},
		TokenURL:                 ts.tokenURL(),
		EndpointParams:           map[string]string{"hi": "hello"},
		Claims: map[string]any{
			"iss":     "https://example.com",
			"aud":     "common-test",
			"sub":     "common",
			"integer": 1,
		},
	}

	var unmarshalledConfig commonconfig.OAuth2
	err := yaml.Unmarshal([]byte(yamlConfig), &unmarshalledConfig)
	require.NoErrorf(t, err, "Expected no error unmarshalling yaml, got %v", err)
	require.Truef(t, reflect.DeepEqual(unmarshalledConfig, expectedConfig), "Got unmarshalled config %v, expected %v", unmarshalledConfig, expectedConfig)

	clientCertificateKey := commonconfig.NewFileSecret(expectedConfig.ClientCertificateKeyFile)
	rt, err := newOAuth2RoundTripper(clientCertificateKey, &expectedConfig, http.DefaultTransport, &defaultHTTPClientOptions)
	require.NoError(t, err)

	client := http.Client{
		Transport: rt,
	}

	resp, err := client.Get(ts.url())
	require.NoError(t, err)

	authorization := resp.Request.Header.Get("Authorization")
	require.Equalf(t, "Bearer 12345", authorization, "Expected authorization header to be 'Bearer', got '%s'", authorization)
}

func TestMarshalURL(t *testing.T) {
	urlp, err := url.Parse("http://example.com/")
	require.NoError(t, err)
	u := &commonconfig.URL{URL: urlp}

	c, err := json.Marshal(u)
	require.NoError(t, err)
	require.Equalf(t, "\"http://example.com/\"", string(c), "URL not properly marshaled in JSON got '%s'", string(c))

	c, err = yaml.Marshal(u)
	require.NoError(t, err)
	require.Equalf(t, "http://example.com/\n", string(c), "URL not properly marshaled in YAML got '%s'", string(c))
}

func TestMarshalURLWrapperWithNilValue(t *testing.T) {
	u := &commonconfig.URL{}

	c, err := json.Marshal(u)
	require.NoError(t, err)
	require.Equalf(t, "null", string(c), "URL with nil value not properly marshaled into JSON, got %q", c)

	c, err = yaml.Marshal(u)
	require.NoError(t, err)
	require.Equalf(t, "null\n", string(c), "URL with nil value not properly marshaled into JSON, got %q", c)
}

func TestUnmarshalNullURL(t *testing.T) {
	b := []byte(`null`)

	{
		var u commonconfig.URL
		err := json.Unmarshal(b, &u)
		require.NoError(t, err)
		require.Truef(t, isEmptyNonNilURL(u.URL), "`null` literal not properly unmarshaled from JSON as URL, got %#v", u.URL)
	}

	{
		var u commonconfig.URL
		err := yaml.Unmarshal(b, &u)
		require.NoError(t, err)
		// UnmarshalYAML is not called when parsing null literal.
		require.Nilf(t, u.URL, "`null` literal not properly unmarshaled from YAML as URL, got %#v", u.URL)
	}
}

func TestUnmarshalEmptyURL(t *testing.T) {
	b := []byte(`""`)

	{
		var u commonconfig.URL
		err := json.Unmarshal(b, &u)
		require.NoError(t, err)
		require.Truef(t, isEmptyNonNilURL(u.URL), "empty string not properly unmarshaled from JSON as URL, got %#v", u.URL)
	}

	{
		var u commonconfig.URL
		err := yaml.Unmarshal(b, &u)
		require.NoError(t, err)
		require.Truef(t, isEmptyNonNilURL(u.URL), "empty string not properly unmarshaled from YAML as URL, got %#v", u.URL)
	}
}

// checks if u equals to &url.URL{}.
func isEmptyNonNilURL(u *url.URL) bool {
	return u != nil && *u == url.URL{}
}

func TestUnmarshalURL(t *testing.T) {
	b := []byte(`"http://example.com/a b"`)
	var u commonconfig.URL

	err := json.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equalf(t, "http://example.com/a%20b", u.String(), "URL not properly unmarshaled in JSON, got '%s'", u.String())

	err = yaml.Unmarshal(b, &u)
	require.NoError(t, err)
	require.Equalf(t, "http://example.com/a%20b", u.String(), "URL not properly unmarshaled in YAML, got '%s'", u.String())
}

func TestMarshalURLWithSecret(t *testing.T) {
	var u commonconfig.URL
	err := yaml.Unmarshal([]byte("http://foo:bar@example.com"), &u) // trufflehog:ignore
	require.NoError(t, err)

	b, err := yaml.Marshal(u)
	require.NoError(t, err)
	require.Equalf(t, "http://foo:xxxxx@example.com", strings.TrimSpace(string(b)), "URL not properly marshaled in YAML, got '%s'", string(b)) // trufflehog:ignore
}

func TestHTTPClientConfig_Marshal(t *testing.T) {
	proxyURL, err := url.Parse("http://localhost:8080")
	require.NoError(t, err)

	t.Run("without HTTP headers", func(t *testing.T) {
		config := &commonconfig.HTTPClientConfig{
			ProxyConfig: commonconfig.ProxyConfig{
				ProxyURL: commonconfig.URL{URL: proxyURL},
			},
		}

		t.Run("YAML", func(t *testing.T) {
			actualYAML, err := yaml.Marshal(config)
			require.NoError(t, err)
			require.YAMLEq(t, `
proxy_url: "http://localhost:8080"
follow_redirects: false
enable_http2: false
`, string(actualYAML))

			// Unmarshalling the YAML should get the same struct in input.
			actual := &commonconfig.HTTPClientConfig{}
			require.NoError(t, yaml.Unmarshal(actualYAML, actual))
			require.Equal(t, config, actual)
		})

		t.Run("JSON", func(t *testing.T) {
			actualJSON, err := json.Marshal(config)

			require.NoError(t, err)
			require.JSONEq(t, `{
				"proxy_url":"http://localhost:8080",
				"tls_config":{"insecure_skip_verify":false},
				"follow_redirects":false,
				"enable_http2":false
			}`, string(actualJSON))

			// Unmarshalling the JSON should get the same struct in input.
			actual := &commonconfig.HTTPClientConfig{}
			require.NoError(t, json.Unmarshal(actualJSON, actual))
			require.Equal(t, config, actual)
		})
	})

	t.Run("with HTTP headers", func(t *testing.T) {
		config := &commonconfig.HTTPClientConfig{
			ProxyConfig: commonconfig.ProxyConfig{
				ProxyURL: commonconfig.URL{URL: proxyURL},
			},
			HTTPHeaders: &commonconfig.Headers{
				Headers: map[string]commonconfig.Header{
					"X-Test": {
						Values: []string{"Value-1", "Value-2"},
					},
				},
			},
		}

		actualYAML, err := yaml.Marshal(config)
		require.NoError(t, err)
		require.YAMLEq(t, `
proxy_url: "http://localhost:8080"
follow_redirects: false
enable_http2: false
http_headers:
  X-Test:
    values:
    - Value-1
    - Value-2
`, string(actualYAML))

		actualJSON, err := json.Marshal(config)
		require.NoError(t, err)
		require.JSONEq(t, `{
			"proxy_url":"http://localhost:8080",
			"tls_config":{"insecure_skip_verify":false},
			"follow_redirects":false,
			"enable_http2":false,
			"http_headers":{"X-Test":{"values":["Value-1","Value-2"]}}
		}`, string(actualJSON))
	})
}

func TestOAuth2Proxy(t *testing.T) {
	_, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.oauth2-proxy.good.yml")
	if err != nil {
		t.Errorf("Error loading OAuth2 client config: %v", err)
	}
}

func TestModifyTLSCertificates(t *testing.T) {
	clientKeyNoPassPath := testdata.ClientKeyNoPassPath(t)

	tmpDir := t.TempDir()
	ca, cert, key := filepath.Join(tmpDir, "ca"), filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	tests := []struct {
		ca   string
		cert string
		key  string

		errMsg string

		modification func()
	}{
		{
			ca:   ClientCertificatePath,
			cert: ClientCertificatePath,
			key:  clientKeyNoPassPath,

			errMsg: "certificate signed by unknown authority",

			modification: func() { writeCertificate(TLSCAChainPath, ca) },
		},
		{
			ca:   TLSCAChainPath,
			cert: WrongClientCertPath,
			key:  clientKeyNoPassPath,

			errMsg: "private key does not match public key",

			modification: func() { writeCertificate(ClientCertificatePath, cert) },
		},
		{
			ca:   TLSCAChainPath,
			cert: ClientCertificatePath,
			key:  WrongClientCertPath,

			errMsg: "found a certificate rather than a key in the PEM for the private key",

			modification: func() { writeCertificate(clientKeyNoPassPath, key) },
		},
	}

	cfg := commonconfig.HTTPClientConfig{
		TLSConfig: commonconfig.TLSConfig{
			CAFile:             ca,
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: false,
		},
	}

	var c *http.Client
	for i, tc := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			writeCertificate(tc.ca, ca)
			writeCertificate(tc.cert, cert)
			writeCertificate(tc.key, key)
			if c == nil {
				c, err = NewClientFromConfig(cfg, "test")
				require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)
			}

			req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
			require.NoErrorf(t, err, "Error creating HTTP request: %v", err)

			r, err := c.Do(req)
			if err == nil {
				r.Body.Close()
				t.Fatalf("Could connect to the test server.")
			}
			require.ErrorContainsf(t, err, tc.errMsg, "Expected error message to contain %q, got %q", tc.errMsg, err)

			tc.modification()

			r, err = c.Do(req)
			require.NoErrorf(t, err, "Expected no error, got %q", err)

			b, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				t.Errorf("Can't read the server response body")
			}

			got := strings.TrimSpace(string(b))
			if ExpectedMessage != got {
				t.Errorf("The expected message %q differs from the obtained message %q", ExpectedMessage, got)
			}
		})
	}
}

func TestTLSRoundTripper_NoCAConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	cert, key := filepath.Join(tmpDir, "cert"), filepath.Join(tmpDir, "key")

	handler := func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, ExpectedMessage)
	}
	testServer, err := newTestServer(t, handler)
	require.NoError(t, err)
	defer testServer.Close()

	cfg := commonconfig.HTTPClientConfig{
		TLSConfig: commonconfig.TLSConfig{
			CertFile:           cert,
			KeyFile:            key,
			InsecureSkipVerify: true,
		},
	}

	writeCertificate(ClientCertificatePath, cert)
	writeCertificate(testdata.ClientKeyNoPassPath(t), key)
	c, err := NewClientFromConfig(cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	req, err := http.NewRequest(http.MethodGet, testServer.URL, nil)
	require.NoErrorf(t, err, "Error creating HTTP request: %v", err)

	r, err := c.Do(req)
	require.NoErrorf(t, err, "Can't connect to the test server")
	r.Body.Close()

	err = os.WriteFile(cert, []byte("-----BEGIN GARBAGE-----\nabc\n-----END GARBAGE-----\n"), 0o664)
	require.NoError(t, err)

	_, err = c.Do(req)
	require.ErrorContainsf(t, err, "unable to use specified CA cert: none configured", "Expected error to mention missing CA cert")
}

// loadHTTPConfigJSON parses the JSON input s into a commonconfig.HTTPClientConfig.
func loadHTTPConfigJSON(buf []byte) (*commonconfig.HTTPClientConfig, error) {
	cfg := &commonconfig.HTTPClientConfig{}
	err := json.Unmarshal(buf, cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadHTTPConfigJSONFile parses the given JSON file into a commonconfig.HTTPClientConfig.
func loadHTTPConfigJSONFile(filename string) (*commonconfig.HTTPClientConfig, []byte, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg, err := loadHTTPConfigJSON(content)
	if err != nil {
		return nil, nil, err
	}
	return cfg, content, nil
}

func TestProxyConfig_Proxy(t *testing.T) {
	var proxyServer *httptest.Server

	defer func() {
		if proxyServer != nil {
			proxyServer.Close()
		}
	}()

	proxyServerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, %s", r.URL.Path)
	})

	proxyServer = httptest.NewServer(proxyServerHandler)

	testCases := []struct {
		name             string
		proxyConfig      string
		expectedProxyURL string
		targetURL        string
		proxyEnv         string
		noProxyEnv       string
	}{
		{
			name:             "proxy from environment",
			proxyConfig:      `proxy_from_environment: true`,
			expectedProxyURL: proxyServer.URL,
			proxyEnv:         proxyServer.URL,
			targetURL:        "http://prometheus.io/",
		},
		{
			name:             "proxy_from_environment with no_proxy",
			proxyConfig:      `proxy_from_environment: true`,
			expectedProxyURL: "",
			proxyEnv:         proxyServer.URL,
			noProxyEnv:       "prometheus.io",
			targetURL:        "http://prometheus.io/",
		},
		{
			name:             "proxy_from_environment and localhost",
			proxyConfig:      `proxy_from_environment: true`,
			expectedProxyURL: "",
			proxyEnv:         proxyServer.URL,
			targetURL:        "http://localhost/",
		},
		{
			name:             "valid proxy_url and localhost",
			proxyConfig:      "proxy_url: " + proxyServer.URL,
			expectedProxyURL: proxyServer.URL,
			targetURL:        "http://localhost/",
		},
		{
			name: "valid proxy_url and no_proxy and localhost",
			proxyConfig: fmt.Sprintf(`proxy_url: %s
no_proxy: prometheus.io`, proxyServer.URL),
			expectedProxyURL: "",
			targetURL:        "http://localhost/",
		},
		{
			name:             "valid proxy_url",
			proxyConfig:      "proxy_url: " + proxyServer.URL,
			expectedProxyURL: proxyServer.URL,
			targetURL:        "http://prometheus.io/",
		},
		{
			name: "valid proxy url and no_proxy",
			proxyConfig: fmt.Sprintf(`proxy_url: %s
no_proxy: prometheus.io`, proxyServer.URL),
			expectedProxyURL: "",
			targetURL:        "http://prometheus.io/",
		},
		{
			name: "valid proxy url and no_proxies",
			proxyConfig: fmt.Sprintf(`proxy_url: %s
no_proxy: promcon.io,prometheus.io,cncf.io`, proxyServer.URL),
			expectedProxyURL: "",
			targetURL:        "http://prometheus.io/",
		},
		{
			name: "valid proxy url and no_proxies that do not include target",
			proxyConfig: fmt.Sprintf(`proxy_url: %s
no_proxy: promcon.io,cncf.io`, proxyServer.URL),
			expectedProxyURL: proxyServer.URL,
			targetURL:        "http://prometheus.io/",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if proxyServer != nil {
				defer proxyServer.Close()
			}

			var proxyConfig commonconfig.ProxyConfig

			err := yaml.Unmarshal([]byte(tc.proxyConfig), &proxyConfig)
			if err != nil {
				t.Errorf("failed to unmarshal proxy config: %v", err)
				return
			}

			if tc.proxyEnv != "" {
				t.Setenv("HTTP_PROXY", tc.proxyEnv)
			}

			if tc.noProxyEnv != "" {
				t.Setenv("NO_PROXY", tc.noProxyEnv)
			}

			req := httptest.NewRequest(http.MethodGet, tc.targetURL, nil)

			proxyFunc := proxyConfig.Proxy()
			resultURL, err := proxyFunc(req)
			if err != nil {
				t.Fatalf("expected no error, but got: %v", err)
				return
			}
			if tc.expectedProxyURL == "" && resultURL != nil {
				t.Fatalf("expected no result URL, but got: %s", resultURL.String())
				return
			}
			if tc.expectedProxyURL != "" && resultURL == nil {
				t.Fatalf("expected result URL, but got nil")
				return
			}
			if tc.expectedProxyURL != "" && resultURL.String() != tc.expectedProxyURL {
				t.Fatalf("expected result URL: %s, but got: %s", tc.expectedProxyURL, resultURL.String())
			}
		})
	}
}

func readFile(t *testing.T, filename string) string {
	t.Helper()

	content, err := os.ReadFile(filename)
	require.NoErrorf(t, err, "Failed to read file %q: %s", filename, err)

	return string(content)
}

func TestHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range map[string]string{
			"One":   "value1",
			"Two":   "value2",
			"Three": "value3",
		} {
			if r.Header.Get(k) != v {
				t.Errorf("expected %q, got %q", v, r.Header.Get(k))
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.headers.good.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	_, err = client.Get(ts.URL)
	require.NoErrorf(t, err, "can't fetch URL: %v", err)
}

func TestMultipleHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range map[string][]string{
			"One":   {"value1a", "value1b", "value1c"},
			"Two":   {"value2a", "value2b", "value2c"},
			"Three": {"value3a", "value3b", "value3c"},
		} {
			if !reflect.DeepEqual(r.Header.Values(k), v) {
				t.Errorf("expected %v, got %v", v, r.Header.Values(k))
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	cfg, _, err := commonconfig.LoadHTTPConfigFile("testdata/http.conf.headers-multiple.good.yaml")
	require.NoErrorf(t, err, "Error loading HTTP client config: %v", err)
	client, err := NewClientFromConfig(*cfg, "test")
	require.NoErrorf(t, err, "Error creating HTTP Client: %v", err)

	_, err = client.Get(ts.URL)
	require.NoErrorf(t, err, "can't fetch URL: %v", err)
}

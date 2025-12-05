package write

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/parca/reporter"
	commonconfig "github.com/prometheus/common/config"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type DebugInfoOptions struct {
	Enabled          bool   `alloy:"enabled,attr,optional"`
	CacheSize        uint32 `alloy:"cache_size,attr,optional"`
	StripTextSection bool   `alloy:"strip_text_section,attr,optional"`
	QueueSize        uint32
	WorkerNum        int
	CachePath        string
}

func newDebugInfoUpload(u *url.URL, metrics *metrics, e *EndpointOptions) (*reporter.ParcaSymbolUploader, error) {
	if !e.DebugInfo.Enabled {
		return nil, nil
	}

	creds := insecure.NewCredentials()

	if promTLSConfig := e.HTTPClientConfig.TLSConfig.Convert(); promTLSConfig != nil {
		tlsConf, err := commonconfig.NewTLSConfig(promTLSConfig)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(tlsConf)
	} else if u.Scheme == "https" {
		creds = credentials.NewTLS(&tls.Config{})
	}
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.HeaderCallOption{
			HeaderAddr: nil,
		}),
	}
	if auth, err := newGrpcBasicAuthCredentials(e); err != nil {
		return nil, err
	} else if auth != nil {
		opts = append(opts, grpc.WithPerRPCCredentials(auth))
	}
	cc, err := grpc.NewClient(fmt.Sprintf("%s:%s", u.Hostname(), u.Port()), opts...)
	if err != nil {
		return nil, err
	}

	return reporter.NewParcaSymbolUploader(
		debuginfogrpc.NewDebuginfoServiceClient(cc),
		e.DebugInfo.CacheSize,
		e.DebugInfo.StripTextSection,
		e.DebugInfo.QueueSize,
		e.DebugInfo.WorkerNum,
		e.DebugInfo.CachePath,
		metrics.debugInfoUploadBytes,
	)
}

func (f *fanOutClient) UploadDebugInfo(ctx context.Context, fileID libpf.FileID, fileName string, buildID string, open func() (process.ReadAtCloser, error)) {
	for _, u := range f.debugInfo {
		u.Upload(context.TODO(), fileID, fileName, buildID, open)
	}
}

func newGrpcBasicAuthCredentials(e *EndpointOptions) (*basicAuthCredential, error) {
	auth := e.HTTPClientConfig.BasicAuth
	if auth == nil || auth.Username == "" {
		return nil, nil
	}
	if auth.Password != "" {
		return &basicAuthCredential{
			username: auth.Username,
			password: string(auth.Password),
		}, nil
	}
	if auth.PasswordFile != "" {
		passwordBytes, err := os.ReadFile(auth.PasswordFile)
		if err != nil {
			return nil, err
		}
		return &basicAuthCredential{
			username: auth.Username,
			password: strings.TrimSpace(string(passwordBytes)),
		}, nil

	}
	return nil, nil
}

type basicAuthCredential struct {
	username string
	password string
}

func (b *basicAuthCredential) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	auth := b.username + ":" + b.password
	encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
	return map[string]string{
		"authorization": "Basic " + encodedAuth,
	}, nil
}

func (b *basicAuthCredential) RequireTransportSecurity() bool {
	return true
}

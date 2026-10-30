package otlp_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	exporterotlp "github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/otlp"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"go.opentelemetry.io/collector/pdata/testdata"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip"
)

type profilesPipelineSink struct {
	pprofileotlp.UnimplementedGRPCServer
	profiles chan pprofile.Profiles
}

func (s *profilesPipelineSink) Export(ctx context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	select {
	case s.profiles <- req.Profiles():
		return pprofileotlp.NewExportResponse(), nil
	case <-ctx.Done():
		return pprofileotlp.NewExportResponse(), ctx.Err()
	}
}

func TestProfilesPipeline(t *testing.T) {
	for _, transport := range []string{"grpc", "http"} {
		t.Run(transport, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(componenttest.TestContext(t), 10*time.Second)
			defer cancel()
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			sink := &profilesPipelineSink{profiles: make(chan pprofile.Profiles, 1)}
			server := grpc.NewServer()
			pprofileotlp.RegisterGRPCServer(server, sink)
			go func() { _ = server.Serve(listener) }()
			defer server.Stop()

			experimental := func(opts component.Options) component.Options {
				opts.MinStability = featuregate.StabilityExperimental
				return opts
			}
			exporter, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.exporter.otlp")
			require.NoError(t, err)
			var exporterArgs exporterotlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(fmt.Sprintf(`
    client {
     endpoint = %q
     tls { insecure = true }
    }
    sending_queue { enabled = false }
    retry_on_failure { enabled = false }
   `, listener.Addr().String())), &exporterArgs))
			go func() { require.NoError(t, exporter.Run(ctx, exporterArgs, experimental)) }()
			require.NoError(t, exporter.WaitRunning(time.Second))
			require.NoError(t, exporter.WaitExports(time.Second))

			receiver, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.receiver.otlp")
			require.NoError(t, err)
			address := componenttest.GetFreeAddr(t)
			var receiverArgs otlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(fmt.Sprintf(`
    %s { endpoint = %q }
    output {}
   `, transport, address)), &receiverArgs))
			receiverArgs.Output.Profiles = []otelcol.Consumer{exporter.Exports().(otelcol.ConsumerExports).Input}
			go func() { require.NoError(t, receiver.Run(ctx, receiverArgs, experimental)) }()
			require.NoError(t, receiver.WaitRunning(time.Second))

			want := testdata.GenerateProfiles(2)
			request := pprofileotlp.NewExportRequestFromProfiles(want)
			if transport == "grpc" {
				conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
				require.NoError(t, err)
				defer conn.Close()
				_, err = pprofileotlp.NewGRPCClient(conn).Export(ctx, request)
				require.NoError(t, err)
			} else {
				body, err := request.MarshalProto()
				require.NoError(t, err)
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+address+"/v1development/profiles", bytes.NewReader(body))
				require.NoError(t, err)
				req.Header.Set("Content-Type", "application/x-protobuf")
				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()
				require.Equal(t, http.StatusOK, resp.StatusCode)
			}
			select {
			case got := <-sink.profiles:
				marshaler := &pprofile.JSONMarshaler{}
				wantJSON, err := marshaler.MarshalProfiles(want)
				require.NoError(t, err)
				gotJSON, err := marshaler.MarshalProfiles(got)
				require.NoError(t, err)
				require.JSONEq(t, string(wantJSON), string(gotJSON))
			case <-ctx.Done():
				t.Fatal("profiles did not reach the OTLP exporter")
			}
		})
	}
}

func TestProfilesReceiverStability(t *testing.T) {
	for _, stability := range []featuregate.Stability{featuregate.StabilityGenerallyAvailable, featuregate.StabilityPublicPreview} {
		t.Run(stability.String(), func(t *testing.T) {
			receiver, err := componenttest.NewControllerFromID(util.TestLogger(t), "otelcol.receiver.otlp")
			require.NoError(t, err)
			var args otlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(`grpc {}
output {}`), &args))
			args.Output.Profiles = []otelcol.Consumer{&fakeconsumer.Consumer{}}
			err = receiver.Run(t.Context(), args, func(opts component.Options) component.Options { opts.MinStability = stability; return opts })
			require.ErrorContains(t, err, "--stability.level")
		})
	}
}

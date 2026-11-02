//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// fakeBeyla answers the config refreshes of a configControl as Beyla would, with the given errors.
func fakeBeyla(t *testing.T, controlSocket net.Conn, errors ...string) <-chan string {
	t.Helper()
	received := make(chan string, len(errors))
	go func() {
		defer controlSocket.Close()
		decoder, encoder := json.NewDecoder(controlSocket), json.NewEncoder(controlSocket)
		for _, rejection := range errors {
			var request configRefreshRequest
			if err := decoder.Decode(&request); err != nil {
				return
			}
			received <- request.Config
			if err := encoder.Encode(configRefreshResponse{Error: rejection}); err != nil {
				return
			}
		}
	}()
	return received
}

func newTestConfigControl(t *testing.T) (*configControl, net.Conn) {
	t.Helper()
	control, controlSocket, err := newConfigControl()
	require.NoError(t, err)
	t.Cleanup(func() { control.Close() })

	child, err := net.FileConn(controlSocket)
	require.NoError(t, err)
	require.NoError(t, controlSocket.Close())
	return control, child
}

func TestConfigControlRefresh(t *testing.T) {
	control, child := newTestConfigControl(t)
	received := fakeBeyla(t, child, "", "service settings changed; restart Beyla")

	require.NoError(t, control.refresh(t.Context(), []byte("discovery: first")))
	assert.Equal(t, "discovery: first", <-received)

	err := control.refresh(t.Context(), []byte("discovery: second"))
	require.ErrorContains(t, err, "service settings changed; restart Beyla")
	assert.Equal(t, "discovery: second", <-received)

	// Beyla closed its end, as when the subprocess exits.
	assert.Error(t, control.refresh(t.Context(), []byte("discovery: third")))
}

func TestConfigControlRefresh_NoAnswer(t *testing.T) {
	control, child := newTestConfigControl(t)
	defer child.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	require.Error(t, control.refresh(ctx, []byte("discovery: unanswered")))
	assert.Less(t, time.Since(start), configRefreshTimeout, "the context must stop the wait")
}

// testConsumer is a comparable consumer.
type testConsumer struct{ otelcol.Consumer }

// funcConsumer is not comparable, so it is never considered to be the same consumer.
type funcConsumer struct {
	otelcol.Consumer
	consume func()
}

func TestSameOutput(t *testing.T) {
	first, second := &testConsumer{}, &testConsumer{}

	for _, tc := range []struct {
		name string
		a, b *otelcol.ConsumerArguments
		same bool
	}{
		{name: "no output", same: true},
		{name: "added output", b: &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first}}},
		{
			name: "same consumers",
			a:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first}, Metrics: []otelcol.Consumer{second}},
			b:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first}, Metrics: []otelcol.Consumer{second}},
			same: true,
		},
		{
			name: "replaced consumer",
			a:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first}},
			b:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{second}},
		},
		{
			name: "added consumer",
			a:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first}},
			b:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{first, second}},
		},
		{
			name: "consumer not comparable",
			a:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{funcConsumer{}}},
			b:    &otelcol.ConsumerArguments{Traces: []otelcol.Consumer{funcConsumer{}}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.same, sameExporters(tc.a, tc.b))
		})
	}
}

package remote_relabel

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	settingsv1 "github.com/grafana/pyroscope/api/gen/proto/go/settings/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

type mockWs struct {
	t         testing.TB
	read      chan []byte
	write     chan []byte
	closeOnce sync.Once
	wg        sync.WaitGroup
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func newMockWs(t testing.TB) *mockWs {
	return &mockWs{
		t:     t,
		read:  make(chan []byte),
		write: make(chan []byte),
	}
}
func (ws *mockWs) close() {
	ws.closeOnce.Do(func() {
		close(ws.write)
	})
	ws.wg.Wait()
}

func (ws *mockWs) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		require.NoError(ws.t, err)
		return
	}

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil && websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				ws.closeOnce.Do(func() {
					close(ws.write)
				})
				close(ws.read)
				break
			}
			require.NoError(ws.t, err, "error reading message")
			ws.read <- msg
		}
	}()

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		defer conn.Close()
		for b := range ws.write {
			err := conn.WriteMessage(websocket.TextMessage, b)
			require.NoError(ws.t, err, "error writing message")
		}
	}()
}

func Test_Instances_Update(t *testing.T) {
	// ensure that all goroutines are cleaned up
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create mock websocket server
	ws, wsURL, cleanup := newWebsocketTest(t)
	defer cleanup()

	argument := Arguments{
		Targets: []discovery.Target{
			map[string]string{"job": "foo"},
			map[string]string{"job": "bar"},
		},
		WebsocketURL: wsURL,
	}

	c, err := New(component.Options{
		ID:            "1",
		Logger:        util.TestAlloyLogger(t),
		OnStateChange: func(e component.Exports) {},
	}, argument)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		require.NoError(t, c.Run(ctx))
		close(done)
	}()

	// subscribe to instance updates
	ws.write <- []byte(`{"payload_subscribe":{"topics":["instances"]}}`)

	// expect instance update
	var msg settingsv1.CollectionMessage
	for read := range ws.read {
		err := json.Unmarshal(read, &msg)
		require.NoError(t, err)

		if msg.PayloadData != nil {
			break
		}
	}

	// assert instance update
	require.NotNil(t, msg.PayloadData)
	require.Len(t, msg.PayloadData.Instances, 1)
	assert.Equal(t, defaultInstance(), msg.PayloadData.Instances[0].Hostname)
	targets, err := json.Marshal(msg.PayloadData.Instances[0].Targets)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"labels":[{"name":"job","value":"foo"}]},{"labels":[{"name":"job","value":"bar"}]}]`, string(targets))

	cancel()
	<-done
	t.Log("component stopped")
}

func newWebsocketTest(t testing.TB) (*mockWs, string, func()) {
	ws := newMockWs(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	srv := httptest.NewServer(mux)
	return ws, strings.Replace(srv.URL, "http", "ws", 1) + "/ws", func() {
		srv.Close()
		ws.close()
	}
}

func Test_Rule_Update(t *testing.T) {
	// ensure that all goroutines are cleaned up
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create mock websocket server
	ws, wsURL, cleanup := newWebsocketTest(t)
	defer cleanup()

	argument := Arguments{
		Targets: []discovery.Target{
			map[string]string{"job": "foo"},
			map[string]string{"job": "bar"},
		},
		WebsocketURL: wsURL,
	}

	exportsCh := make(chan Exports, 1)
	c, err := New(component.Options{
		ID:     "1",
		Logger: util.TestAlloyLogger(t),
		OnStateChange: func(e component.Exports) {
			export, ok := e.(Exports)
			require.True(t, ok, "expect correct type")
			exportsCh <- export
		},
	}, argument)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		require.NoError(t, c.Run(ctx))
		close(done)
	}()

	msg := <-ws.read
	require.JSONEq(t, `{"payload_subscribe":{"topics":["rules"]}}`, string(msg))

	// Send a initial rule
	ws.write <- []byte(fmt.Sprintf(`{"payload_data":{"rules":[{"action": %d}]}}`, settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_DROP))

	// retrieve updates
	for export := range exportsCh {
		if len(export.Rules) > 0 {
			// export as alloy syntax
			v, err := syntax.Marshal(export)
			require.NoError(t, err)
			assert.Equal(t, `websocket_status = "connected"
output           = []
rules            = [{
	source_labels = [],
	separator     = ";",
	regex         = "(.*)",
	modulus       = 0,
	target_label  = "",
	replacement   = "$1",
	action        = "drop",
}]`, string(v))
			t.Log("initial rule received")
			break
		}
	}

	// Send a new rule
	ws.write <- []byte(fmt.Sprintf(`{"payload_data":{"rules":[{"action": %d, "source_labels":["job"], "regex": "foo"}]}}`, settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_KEEP))

	// retrieve updates
	for export := range exportsCh {
		if len(export.Rules) > 0 {
			// export as alloy syntax
			v, err := syntax.Marshal(export)
			require.NoError(t, err)
			assert.Equal(t, `websocket_status = "connected"
output           = [{
	job = "foo",
}]
rules = [{
	source_labels = ["job"],
	separator     = ";",
	regex         = "foo",
	modulus       = 0,
	target_label  = "",
	replacement   = "$1",
	action        = "keep",
}]`, string(v))
			t.Log("second rule received")
			break
		}
	}

	cancel()
	<-done
	t.Log("component stopped")
}

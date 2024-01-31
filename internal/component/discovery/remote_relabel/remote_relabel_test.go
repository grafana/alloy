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

type mockServer struct {
	t    testing.TB
	wg   sync.WaitGroup
	wsCh chan *mockConn
}

func (ws *mockServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		require.NoError(ws.t, err)
		return
	}

	c := &mockConn{
		Conn:  conn,
		t:     ws.t,
		read:  make(chan []byte),
		write: make(chan []byte),
	}

	ws.wsCh <- c

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				c.closeOnce.Do(func() {
					close(c.write)
				})
				close(c.read)
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure) && c.t != nil {
					require.NoError(c.t, err, "error reading message")
				}
				break
			}
			c.read <- msg
		}
	}()

	ws.wg.Add(1)
	go func() {
		defer ws.wg.Done()
		defer c.Close()
		for b := range c.write {
			err := c.WriteMessage(websocket.TextMessage, b)
			if err != nil {
				if c.t != nil {
					require.NoError(c.t, err, "error writing message")
				}
				break
			}
		}
	}()
}

func (ws *mockServer) close() {
	ws.wg.Wait()
}

type mockConn struct {
	*websocket.Conn
	t         testing.TB
	closeOnce sync.Once
	read      chan []byte
	write     chan []byte
}

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func newMockServer(t testing.TB) *mockServer {
	return &mockServer{
		t:    t,
		wsCh: make(chan *mockConn, 16),
	}
}

func Test_Instances_Update(t *testing.T) {
	// ensure that all goroutines are cleaned up
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create mock websocket server
	wsCh, wsURL, cleanup := newWebsocketTest(t)
	defer cleanup()

	argument := Arguments{
		Targets: []discovery.Target{
			map[string]string{"job": "foo"},
			map[string]string{"job": "bar"},
		},
		Websocket: &WebsocketOptions{URL: wsURL},
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

	// get websocket connection
	ws := <-wsCh

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

func newWebsocketTest(t testing.TB) (chan *mockConn, string, func()) {
	ws := newMockServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ws.handleWebSocket)
	srv := httptest.NewServer(mux)
	return ws.wsCh, strings.Replace(srv.URL, "http", "ws", 1) + "/ws", func() {
		srv.Close()
		ws.close()
	}
}

func Test_Rule_Update(t *testing.T) {
	// ensure that all goroutines are cleaned up
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create mock websocket server
	wsCh, wsURL, cleanup := newWebsocketTest(t)
	defer cleanup()

	argument := Arguments{
		Targets: []discovery.Target{
			map[string]string{"job": "foo"},
			map[string]string{"job": "bar"},
		},
		Websocket: &WebsocketOptions{URL: wsURL},
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

	ws := <-wsCh

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
			assert.Equal(t, `output = []
rules  = [{
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
			assert.Equal(t, `output = [{
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

func Test_Reconnect_Websocket(t *testing.T) {
	// ensure that all goroutines are cleaned up
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	// create mock websocket server
	wsCh, wsURL, cleanup := newWebsocketTest(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	argument := Arguments{
		Websocket: &WebsocketOptions{URL: wsURL},
	}

	c, err := New(component.Options{
		ID:            "1",
		Logger:        util.TestAlloyLogger(t),
		OnStateChange: func(e component.Exports) {},
	}, argument)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		require.NoError(t, c.Run(ctx))
		close(done)
	}()

	ws := <-wsCh

	// wait for websocket connection
	msg := <-ws.read
	require.JSONEq(t, `{"payload_subscribe":{"topics":["rules"]}}`, string(msg))

	// unexpectedly close websocket connection
	ws.t = nil // this prevents error reporting
	require.NoError(t, ws.Conn.UnderlyingConn().Close())

	// wait for new websocket
	ws = <-wsCh

	// wait for second websocket initiation
	msg = <-ws.read
	require.JSONEq(t, `{"payload_subscribe":{"topics":["rules"]}}`, string(msg))

	// get debug info
	di := c.DebugInfo()
	diType, ok := di.(debugInfo)
	require.True(t, ok)
	require.Equal(t, "websocket: close 1006 (abnormal closure): unexpected EOF", diType.WebsocketLastError)

	cancel()
	<-done
}

func syntaxParse(obj interface{}, lines ...string) error {
	lines = append(lines, "") // ensure we end with a newline
	return syntax.Unmarshal([]byte(strings.Join(lines, "\n")), obj)
}

func TestValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var args Arguments
		err := syntaxParse(&args,
			`websocket {`,
			`  url = "wss://blabala.com"`,
			`}`,
			`targets = [{job = "foo"}]`,
		)
		require.Nil(t, err)
		assert.Equal(t, "wss://blabala.com", args.Websocket.URL)
		assert.Equal(t, []discovery.Target{{"job": "foo"}}, args.Targets)
	})

	t.Run("missing websockets url", func(t *testing.T) {
		var args Arguments
		err := syntaxParse(&args,
			`targets = [{job = "foo"}]`,
			`websocket {`,
			`}`,
		)
		require.ErrorContains(t, err, `missing required attribute "url"`)
	})

	t.Run("url with wrong scheme", func(t *testing.T) {
		var args Arguments
		err := syntaxParse(&args,
			`targets = [{job = "foo"}]`,
			`websocket {`,
			`  url = "http://blabala.com"`,
			`}`,
		)
		require.ErrorContains(t, err, `websocket.url has invalid scheme "http": expect "ws" or "wss"`)
	})

	t.Run("missing targets required", func(t *testing.T) {
		var args Arguments
		err := syntaxParse(&args,
			`websocket {`,
			`  url = "wss://blabala.com"`,
			`}`,
		)
		require.ErrorContains(t, err, `missing required attribute "targets"`)
	})

	t.Run("backoff too small", func(t *testing.T) {
		var args Arguments
		err := syntaxParse(&args,
			`websocket {`,
			`  url = "wss://blabala.com"`,
			`  min_backoff_period = "1ms"`,
			`  max_backoff_period = "100ms"`,
			`}`,
			`targets = [{job = "foo"}]`,
			"",
		)
		require.ErrorContains(t, err, `websocket.min_backoff_period must be at least 100ms`)
		require.ErrorContains(t, err, `websocket.max_backoff_period must be at least 5s`)
	})
}

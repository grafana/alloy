package remote_relabel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/go-kit/log"
	"github.com/gorilla/websocket"
	ws "github.com/gorilla/websocket"
	"github.com/grafana/dskit/backoff"
	settingsv1 "github.com/grafana/pyroscope/api/gen/proto/go/settings/v1"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/config"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	topicRules     = "rules"
	topicInstances = "instances"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.remote_relabel",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the discovery.remote_relabel component.
type Arguments struct {
	// Targets contains the input 'targets' passed by a service discovery component.
	Targets []discovery.Target `alloy:"targets,attr"`

	// The websocket endpoint options, must be set
	Websocket *WebsocketOptions `alloy:"websocket,block"`
}

type WebsocketOptions struct {
	URL               string            `alloy:"url,attr"`
	BasicAuth         *config.BasicAuth `alloy:"basic_auth,block,optional"`
	KeepAlive         time.Duration     `alloy:"keep_alive,attr,optional"`          // 0 means disabled
	MinBackoff        time.Duration     `alloy:"min_backoff_period,attr,optional"`  // start backoff at this level
	MaxBackoff        time.Duration     `alloy:"max_backoff_period,attr,optional"`  // increase exponentially to this level
	MaxBackoffRetries int               `alloy:"max_backoff_retries,attr,optional"` // give up after this many; zero means infinite retries
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	return a.Websocket.Validate("websocket.")
}

func (w *WebsocketOptions) Validate(prefix string) error {
	var merr error

	if w.MinBackoff < 100*time.Millisecond {
		merr = errors.Join(merr, fmt.Errorf("%smin_backoff_period must be at least 100ms", prefix))
	}

	if w.MaxBackoff < 5*time.Second {
		merr = errors.Join(merr, fmt.Errorf("%smax_backoff_period must be at least 5s", prefix))
	}

	if w.MinBackoff > w.MaxBackoff {
		merr = errors.Join(merr, fmt.Errorf("%smin_backoff_period must be smaller or equal than %smax_backoff_period", prefix, prefix))
	}

	if w.KeepAlive < 5*time.Second && w.KeepAlive != 0 {
		merr = errors.Join(merr, fmt.Errorf("%skeep_alive must be disabled or at least 5s", prefix))
	}

	if w.MaxBackoffRetries < 0 {
		merr = errors.Join(merr, fmt.Errorf("%smax_backoff_retries must be bigger or equals to 0", prefix))
	}

	if u, err := url.Parse(w.URL); err != nil {
	} else if u.Scheme != "ws" && u.Scheme != "wss" {
		return fmt.Errorf(`%surl has invalid scheme "%s": expect "ws" or "wss"`, prefix, u.Scheme)
	}
	return merr
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	a.Websocket = &WebsocketOptions{
		KeepAlive:  295 * time.Second, // Just under 5 minutes
		MinBackoff: 500 * time.Millisecond,
		MaxBackoff: 10 * time.Minute,
	}
}

// Exports holds values which are exported by the discovery.remote_relabel component.
type Exports struct {
	Output []discovery.Target      `alloy:"output,attr"`
	Rules  []*alloy_relabel.Config `alloy:"rules,attr"`
}

type debugInfo struct {
	WebsocketStatus         string    `alloy:"websocket_status,attr,optional"`
	WebsocketConnectedSince time.Time `alloy:"websocket_connected_since,attr,optional"`
	WebsocketLastError      string    `alloy:"websocket_last_error,attr,optional"`
}

type channelWrapper[T any] struct {
	done chan struct{}
	item T
}

func (c *channelWrapper[T]) Set(item T) {
	c.item = item
	close(c.done)
}

func (c *channelWrapper[T]) Wait(ch chan *channelWrapper[T]) T {
	ch <- c
	<-c.done
	return c.item
}

func newChannelWrapper[T any]() *channelWrapper[T] {
	return &channelWrapper[T]{
		done: make(chan struct{}),
	}
}

// Component implements the discovery.remote_relabel component.
type Component struct {
	opts     component.Options
	logger   log.Logger
	instance string

	args   Arguments
	argsCh chan Arguments

	rcs   []*alloy_relabel.Config
	rcsCh chan []*alloy_relabel.Config

	debugInfoCh chan *channelWrapper[debugInfo]                      // this is debug info for the UI
	targetsCh   chan *channelWrapper[[]*settingsv1.CollectionTarget] // over this channel the websocket requests the active targets

	websocket        *webSocket
	websocketStatus  webSocketStatus
	websocketNextTry time.Time

	backoff       *backoff.Backoff
	backoffConfig backoff.Config

	lblBuilder labels.Builder
}

type webSocket struct {
	logger log.Logger
	comp   *Component

	c          *ws.Conn
	url        string
	reqHeaders http.Header
	keepAlive  time.Duration

	lck         sync.Mutex
	connected   bool
	lastErr     error
	sendTargets bool
	targetsCh   chan *channelWrapper[[]*settingsv1.CollectionTarget] // over this channel the websocket requests the active targets

	hash   uint64
	hasher xxhash.Digest

	wg           sync.WaitGroup
	cleanStopCh  chan struct{} // This channel gets closed when a clean stop is requested
	readLoopDone chan struct{} // This channel gets closed when the read loop is done
	out          chan []byte
}

type webSocketState uint8

const (
	webSocketDisconnected webSocketState = iota
	webSocketConnecting
	webSocketConnected
)

func (w webSocketState) String() string {
	switch w {
	case webSocketDisconnected:
		return "disconnected"
	case webSocketConnecting:
		return "connecting"
	case webSocketConnected:
		return "connected"
	default:
		return "unknown"
	}
}

type webSocketStatus struct {
	connectedSince time.Time
	state          webSocketState
	lastError      error
}

func (w *webSocket) setState(connected bool, err error) {
	w.lck.Lock()
	defer w.lck.Unlock()

	w.connected = connected
	if err != nil {
		w.lastErr = err
	}
}
func (w *webSocket) publishTargets() {
	w.lck.Lock()
	defer w.lck.Unlock()

	if !w.sendTargets {
		return
	}

	t := newChannelWrapper[[]*settingsv1.CollectionTarget]().Wait(w.targetsCh)

	if len(t) <= 0 {
		return
	}

	var p settingsv1.CollectionPayloadData
	p.Instances = append(p.Instances, &settingsv1.CollectionInstance{
		Hostname:    w.comp.instance,
		Targets:     t,
		LastUpdated: time.Now().UnixMilli(),
	})

	data, err := json.Marshal(&settingsv1.CollectionMessage{
		PayloadData: &p,
	})
	if err != nil {
		panic(err)
	}

	w.hasher.Reset()
	_, _ = w.hasher.Write(data)
	if hash := w.hasher.Sum64(); w.hash == hash {
		return
	} else {
		w.hash = hash
	}

	level.Debug(w.logger).Log("msg", "publish targets to the control server", "targets", len(t))
	w.out <- data
}

func headersEqual(map1, map2 http.Header) bool {
	if len(map1) != len(map2) {
		return false
	}

	for key, value := range map1 {
		if val, ok := map2[key]; !ok || !slices.Equal(val, value) {
			return false
		}
	}
	return true
}

func (w *webSocket) needsReplace(url string, headers http.Header, keepAlive time.Duration) bool {
	if url != w.url {
		return true
	}
	if keepAlive != w.keepAlive {
		return true
	}
	return !headersEqual(headers, w.reqHeaders)
}

func (w *webSocket) isConnected() (bool, error) {
	w.lck.Lock()
	defer w.lck.Unlock()
	return w.connected, w.lastErr
}

func (w *webSocket) readLoop() error {
	var msg settingsv1.CollectionMessage
	defer w.wg.Done()
	defer close(w.readLoopDone)
	for {
		msg.Reset()
		err := w.c.ReadJSON(&msg)
		if err != nil {
			if ws.IsUnexpectedCloseError(err, ws.CloseNormalClosure) {
				return err
			}
			return nil
		}

		if msg.PayloadData != nil {
			rcs, err := collectionRulesToAlloyRelabelConfigs(msg.PayloadData.Rules)
			if err != nil {
				level.Error(w.logger).Log("msg", "error converting rules to relabel configs", "err", err)
				continue
			}
			w.comp.rcsCh <- rcs
		} else if msg.PayloadSubscribe != nil {
			sendTargets := false
			for _, t := range msg.PayloadSubscribe.Topics {
				if t == topicInstances {
					level.Debug(w.logger).Log("msg", "enable sending targets to control server")
					sendTargets = true
					break
				}
			}
			w.lck.Lock()
			w.sendTargets = sendTargets
			w.lck.Unlock()
			w.publishTargets()
		} else {
			level.Error(w.logger).Log("msg", "unknown message type", "msg", msg.Id)
			continue
		}
	}
}

func (w *webSocket) writeLoop() error {
	var keepAlive <-chan time.Time
	if w.keepAlive > 0 {
		tick := time.NewTicker(30 * time.Second)
		defer tick.Stop()
		keepAlive = tick.C
	} else {
		keepAlive = make(chan time.Time)
	}

	for {
		select {
		case <-w.readLoopDone:
			return nil
		case <-keepAlive:
			err := w.c.WriteMessage(websocket.PingMessage, []byte(fmt.Sprintf("keepalive=%d", time.Now().Unix())))
			if err != nil {
				return fmt.Errorf("failed to send keep alive: %w", err)
			}
		case t := <-w.out:
			wr, err := w.c.NextWriter(ws.TextMessage)
			if err != nil {
				level.Error(w.logger).Log("msg", "error creating writer", "err", err)
				continue
			}
			_, err = wr.Write(t)
			err2 := wr.Close()
			if err != nil {
				level.Error(w.logger).Log("msg", "error writing message", "err", err)
				return err
			}
			if err2 != nil {
				level.Error(w.logger).Log("msg", "error closing writer", "err", err2)
				continue
			}

		case <-w.cleanStopCh:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := w.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return err
			}
			select {
			case <-w.readLoopDone:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

func (w *webSocket) Close() error {
	close(w.cleanStopCh)
	w.wg.Wait()

	return w.c.Close()
}

var _ component.Component = (*Component)(nil)

func defaultInstance() string {
	// TODO: This should come from Alloy
	hostname := os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// New creates a new discovery.remote_relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		instance: defaultInstance(),
		opts:     o,
		logger:   log.With(o.Logger, "component", "discovery.remote_relabel"),

		argsCh:      make(chan Arguments, 1),
		rcsCh:       make(chan []*alloy_relabel.Config),
		debugInfoCh: make(chan *channelWrapper[debugInfo]),
		targetsCh:   make(chan *channelWrapper[[]*settingsv1.CollectionTarget]),
	}

	// Call to Update() to set the output once at the start
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) createWebSocket(urlString string, reqHeaders http.Header, keepAlive time.Duration) (*webSocket, error) {
	w := &webSocket{
		comp:       c,
		logger:     c.logger,
		url:        urlString,
		reqHeaders: reqHeaders,
		keepAlive:  keepAlive,
		targetsCh:  c.targetsCh,

		connected:    true,
		cleanStopCh:  make(chan struct{}),
		readLoopDone: make(chan struct{}),
		out:          make(chan []byte, 16),
	}

	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return nil, fmt.Errorf("invalid websocket URL scheme: %s, must be ws or wss", u.Scheme)
	}
	w.logger = log.With(w.logger, "server_url", u.String())
	level.Debug(w.logger).Log("msg", "connecting to control server websocket")

	var resp *http.Response
	w.c, resp, err = ws.DefaultDialer.Dial(u.String(), reqHeaders)
	if err != nil {
		if resp != nil {
			message := resp.Status
			rBody, rErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
			if rErr == nil {
				message = string(rBody)
			}
			return nil, fmt.Errorf("error dialing websocket status_code=%d message=%s: %w", resp.StatusCode, message, err)
		}
		return nil, err
	}

	pingHandler := w.c.PingHandler()
	w.c.SetPingHandler(func(appData string) error {
		level.Debug(w.logger).Log("msg", "ping", "data", appData)
		return pingHandler(appData)
	})
	w.c.SetPongHandler(func(appData string) error {
		level.Debug(w.logger).Log("msg", "pong", "data", appData)
		return nil
	})

	// subscribe to the rules and instances topics
	var subscribe settingsv1.CollectionMessage
	subscribe.PayloadSubscribe = &settingsv1.CollectionPayloadSubscribe{
		Topics: []string{topicRules},
	}
	msg, err := json.Marshal(&subscribe)
	if err != nil {
		return nil, err
	}
	w.out <- msg

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		if err := w.writeLoop(); err != nil {
			level.Error(w.logger).Log("msg", "error in the write loop", "err", err)
		}
		w.setState(false, err)
	}()

	w.wg.Add(1)
	go func() {
		err := w.readLoop()
		if err != nil {
			level.Error(w.logger).Log("msg", "error in the read loop", "err", err)
		}
		w.setState(false, err)
	}()

	return w, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	t := time.NewTicker(200 * time.Millisecond)
	defer t.Stop()
	defer func() {
		// stop the websocket
		if c.websocket != nil {
			if err := c.websocket.Close(); err != nil {
				level.Error(c.logger).Log("msg", "error closing websocket", "err", err)
			}
		}
	}()
	// close all channels
	defer func() {
		close(c.argsCh)
		close(c.rcsCh)
		close(c.debugInfoCh)
		close(c.targetsCh)
	}()

	// TODO: Implement websocket keep alive

	for {
		select {
		case <-ctx.Done():
			return nil
		case newArgs := <-c.argsCh:
			c.args = newArgs
			c.updateOutputs()
			c.updateWebSocket()
		case newRCS := <-c.rcsCh:
			c.rcs = newRCS
			c.updateOutputs()
		case <-t.C:
			c.updateWebSocket()
		case t := <-c.targetsCh:
			t.Set(mapToTargets(c.args.Targets))
		case di := <-c.debugInfoCh:
			errText := ""
			if err := c.websocketStatus.lastError; err != nil {
				errText = err.Error()
			}
			di.Set(debugInfo{
				WebsocketStatus:         c.websocketStatus.state.String(),
				WebsocketConnectedSince: c.websocketStatus.connectedSince,
				WebsocketLastError:      errText,
			})
		}
	}
}

// this is the main business logic which will reevaluate the exports when either rules or targets have changed.
func (c *Component) updateOutputs() {
	c.opts.OnStateChange(Exports{
		Output: c.filterTargets(),
		Rules:  c.rcs,
	})
}

// this is the side logic, to keep the websocket connected.
func (c *Component) updateWebSocket() {
	// ensure we have a backoff configured with the same values as the arguments
	backoffChanged := false
	if c.backoffConfig.MaxBackoff != c.args.Websocket.MaxBackoff {
		c.backoffConfig.MaxBackoff = c.args.Websocket.MaxBackoff
		backoffChanged = true
	}
	if c.backoffConfig.MinBackoff != c.args.Websocket.MinBackoff {
		c.backoffConfig.MinBackoff = c.args.Websocket.MinBackoff
		backoffChanged = true
	}
	if c.backoffConfig.MaxRetries != c.args.Websocket.MaxBackoffRetries {
		c.backoffConfig.MaxRetries = c.args.Websocket.MaxBackoffRetries
		backoffChanged = true
	}
	// setup backoff if empty or changed
	if c.backoff == nil || backoffChanged {
		c.backoff = backoff.New(context.Background(), c.backoffConfig)
		c.websocketNextTry = time.Time{}
	}

	// web socket is disconnected, create a new one
	if c.websocket == nil {
		c.newWebSocket()
		return
	}

	// check if websocket arguments suggest it needs.
	if c.websocket.needsReplace(c.args.Websocket.URL, c.newHeader(), c.args.Websocket.KeepAlive) {
		c.replaceWebSocket()
		return
	}

	// check if websocket is still connected
	if connected, err := c.websocket.isConnected(); !connected {
		c.websocketStatus.lastError = err
		c.replaceWebSocket()
	}

	// for all we know, the websocket is still connected
}

func (c *Component) newHeader() http.Header {
	req := &http.Request{Header: make(http.Header)}
	if c.args.Websocket.BasicAuth != nil {
		req.SetBasicAuth(c.args.Websocket.BasicAuth.Username, string(c.args.Websocket.BasicAuth.Password))
	}
	return req.Header
}

func (c *Component) newWebSocket() {
	c.websocketStatus.state = webSocketConnecting

	if !c.websocketNextTry.IsZero() && c.websocketNextTry.After(time.Now()) {
		// we are in backoff, don't try to connect
		return
	}

	req := &http.Request{Header: make(http.Header)}
	if c.args.Websocket.BasicAuth != nil {
		req.SetBasicAuth(c.args.Websocket.BasicAuth.Username, string(c.args.Websocket.BasicAuth.Password))
	}
	w, err := c.createWebSocket(c.args.Websocket.URL, c.newHeader(), c.args.Websocket.KeepAlive)
	if err != nil {
		level.Error(c.logger).Log("msg", "error creating websocket", "err", err)
		c.websocketNextTry = time.Now().Add(c.backoff.NextDelay())
		c.websocketStatus.lastError = err
		return
	}
	c.websocket = w
	c.websocketStatus.state = webSocketConnected
	c.websocketStatus.connectedSince = time.Now()
	c.backoff.Reset()
	c.websocketNextTry = time.Time{}
}

func (c *Component) replaceWebSocket() {
	if err := c.websocket.Close(); err != nil {
		level.Error(c.logger).Log("msg", "error closing websocket", "err", err)
	}
	c.websocketStatus.state = webSocketDisconnected
	c.websocket = nil
	c.newWebSocket()
}

func (c *Component) filterTargets() []discovery.Target {
	rcs := alloy_relabel.ComponentToPromRelabelConfigs(c.rcs)
	targets := make([]discovery.Target, 0, len(c.args.Targets))
	for _, t := range c.args.Targets {
		c.lblBuilder.Reset(nil)
		addTargetToLblBuilder(&c.lblBuilder, t)
		if keep := relabel.ProcessBuilder(&c.lblBuilder, rcs...); keep {
			targets = append(targets, lblBuilderToMap(&c.lblBuilder))
		}
	}
	return targets
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)
	c.argsCh <- newArgs
	return nil
}

// DebugInfo returns debug information for this component.
func (c *Component) DebugInfo() interface{} {
	return newChannelWrapper[debugInfo]().Wait(c.debugInfoCh)
}

// CurrentHealth returns the health of the component.
func (c *Component) CurrentHealth() component.Health {
	debugInfo := newChannelWrapper[debugInfo]().Wait(c.debugInfoCh)

	h := component.Health{
		Health:     component.HealthTypeUnknown,
		UpdateTime: time.Now(),
	}

	if debugInfo.WebsocketStatus == webSocketConnected.String() {
		h.Health = component.HealthTypeHealthy
	} else {
		h.Health = component.HealthTypeUnhealthy
		h.Message = debugInfo.WebsocketLastError
	}
	return h
}

func collectionRulesToAlloyRelabelConfigs(rules []*settingsv1.CollectionRule) (alloy_relabel.Rules, error) {
	res := make(alloy_relabel.Rules, 0, len(rules))
	resX := make([]alloy_relabel.Config, len(rules))
	for idx := range rules {
		if err := collectionRulesToAlloyRelabelConfig(rules[idx], &resX[idx]); err != nil {
			return nil, err
		}
		res = append(res, &resX[idx])
	}
	return res, nil
}

func lblBuilderToMap(lb *labels.Builder) discovery.Target {
	lbls := lb.Labels()
	res := make(map[string]string, len(lbls))
	for _, l := range lbls {
		res[l.Name] = l.Value
	}
	return res
}

func addTargetToLblBuilder(lb *labels.Builder, t discovery.Target) {
	for k, v := range t {
		lb.Set(k, v)
	}
}

func collectionRulesToAlloyRelabelConfig(rule *settingsv1.CollectionRule, config *alloy_relabel.Config) error {
	// set default values
	config.SetToDefault()

	switch rule.Action {
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_REPLACE:
		config.Action = alloy_relabel.Replace
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_KEEP:
		config.Action = alloy_relabel.Keep
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_DROP:
		config.Action = alloy_relabel.Drop
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_KEEP_EQUAL:
		config.Action = alloy_relabel.KeepEqual
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_DROP_EQUAL:
		config.Action = alloy_relabel.DropEqual
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_HASHMOD:
		config.Action = alloy_relabel.HashMod
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_LABELMAP:
		config.Action = alloy_relabel.LabelMap
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_LABELDROP:
		config.Action = alloy_relabel.LabelDrop
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_LABELKEEP:
		config.Action = alloy_relabel.LabelKeep
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_LOWERCASE:
		config.Action = alloy_relabel.Lowercase
	case settingsv1.CollectionRuleAction_COLLECTION_RULE_ACTION_UPPERCASE:
		config.Action = alloy_relabel.Uppercase
	default:
		return fmt.Errorf("unknown action %s", rule.Action)
	}

	if rule.Modulus != nil {
		config.Modulus = *rule.Modulus
	}

	if rule.Regex != nil {
		var err error
		config.Regex, err = alloy_relabel.NewRegexp(*rule.Regex)
		if err != nil {
			return err
		}
	}

	if rule.Replacement != nil {
		config.Replacement = *rule.Replacement
	}

	if rule.Separator != nil {
		config.Separator = *rule.Separator
	}

	if rule.TargetLabel != nil {
		config.TargetLabel = *rule.TargetLabel
	}

	if len(rule.SourceLabels) > 0 {
		config.SourceLabels = rule.SourceLabels
	}

	return nil
}

func mapToTargets(m []discovery.Target) []*settingsv1.CollectionTarget {
	result := make([]*settingsv1.CollectionTarget, len(m))
	var labelNames []string
	for idx := range m {
		labelNames = labelNames[:0]
		for k := range m[idx] {
			labelNames = append(labelNames, k)
		}
		sort.Strings(labelNames)
		result[idx] = &settingsv1.CollectionTarget{
			Labels: make([]*typesv1.LabelPair, len(labelNames)),
		}
		for j, k := range labelNames {
			result[idx].Labels[j] = &typesv1.LabelPair{
				Name:  k,
				Value: m[idx][k],
			}
		}
	}
	return result
}

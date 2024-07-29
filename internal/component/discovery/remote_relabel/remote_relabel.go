package remote_relabel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/go-kit/log"
	"github.com/gorilla/websocket"
	ws "github.com/gorilla/websocket"
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

	// The websocket URL to connect to.
	// This is a required field and must start with either 'ws://' or 'wss://'.
	WebsocketURL string `alloy:"websocket_url,attr"`

	WebsocketBasicAuth *config.BasicAuth `alloy:"websocket_basic_auth,block,optional"`
}

// Exports holds values which are exported by the discovery.remote_relabel component.
type Exports struct {
	WebsocketStatus string                  `alloy:"websocket_status,attr"`
	Output          []discovery.Target      `alloy:"output,attr"`
	Rules           []*alloy_relabel.Config `alloy:"rules,attr"`
}

// Component implements the discovery.remote_relabel component.
type Component struct {
	opts     component.Options
	logger   log.Logger
	instance string

	websocket       *webSocket
	websocketStatus string

	mut         sync.RWMutex
	rcs         []*alloy_relabel.Config
	prevTargets []discovery.Target
	lblBuilder  labels.Builder
}

type webSocket struct {
	logger log.Logger
	comp   *Component

	c   *ws.Conn
	url string

	lck         sync.Mutex
	sendTargets bool
	hash        uint64
	hasher      xxhash.Digest

	wg     sync.WaitGroup
	stopCh chan struct{}
	done   chan struct{}
	out    chan []byte
}

func (w *webSocket) publishTargets(targets []map[string]string) {
	w.lck.Lock()
	defer w.lck.Unlock()

	if !w.sendTargets {
		return
	}

	if len(targets) <= 0 {
		return
	}

	var p settingsv1.CollectionPayloadData
	p.Instances = append(p.Instances, &settingsv1.CollectionInstance{
		Hostname:    w.comp.instance,
		Targets:     mapToTargets(targets),
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

	level.Debug(w.logger).Log("msg", "publish targets to the control server", "targets", len(targets))
	w.out <- data
}

func (w *webSocket) writeLoop() error {
	for {
		select {
		case <-w.done:
			return nil
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

		case <-w.stopCh:
			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := w.c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return err
			}
			select {
			case <-w.done:
			case <-time.After(time.Second):
			}
			return nil
		}
	}
}

func (w *webSocket) Close() error {
	close(w.stopCh)
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
		instance:        defaultInstance(),
		opts:            o,
		logger:          log.With(o.Logger, "component", "discovery.remote_relabel"),
		websocketStatus: "connecting",
	}

	// Call to Update() to set the output once at the start
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) newWebSocket(urlString string, reqHeaders http.Header) (*webSocket, error) {
	w := &webSocket{
		comp:   c,
		logger: c.logger,
		url:    urlString,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
		out:    make(chan []byte, 16),
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
	}()

	w.wg.Add(1)
	go func() {
		var msg settingsv1.CollectionMessage
		defer w.wg.Done()
		defer close(w.done)
		for {
			msg.Reset()
			err := w.c.ReadJSON(&msg)
			if err != nil {
				if ws.IsUnexpectedCloseError(err, ws.CloseNormalClosure) {
					level.Error(w.logger).Log("msg", "error reading JSON message", "err", err)
				}
				return
			}

			if msg.PayloadData != nil {
				rcs, err := collectionRulesToAlloyRelabelConfigs(msg.PayloadData.Rules)
				if err != nil {
					level.Error(w.logger).Log("msg", "error converting rules to relabel configs", "err", err)
					continue
				}
				c.updateRules(rcs)
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
				w.comp.mut.RLock()
				targets := copyTargets(w.comp.prevTargets)
				w.comp.mut.RUnlock()
				w.publishTargets(targets)
			} else {
				level.Error(w.logger).Log("msg", "unknown message type", "msg", msg.Id)
				continue
			}
		}
	}()

	return w, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return c.Stop()
}

func (c *Component) Stop() error {
	var w *webSocket
	c.mut.Lock()
	w = c.websocket
	c.mut.Unlock()

	if w != nil {
		return c.websocket.Close()
	}
	return nil
}

func (c *Component) updateRules(rcs alloy_relabel.Rules) {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.rcs = rcs

	c.opts.OnStateChange(Exports{
		WebsocketStatus: c.websocketStatus,
		Output:          c.underLockFilterTargets(c.prevTargets),
		Rules:           c.rcs,
	})
}

func (c *Component) websocketCloseHandler(code int, text string) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.websocket = nil
	c.websocketStatus = fmt.Sprintf("disconnected: code=%d text=%s", code, text)
	return nil
}

// this requires to be holiding the lock, as it is use the label builder
func (c *Component) underLockFilterTargets([]discovery.Target) []discovery.Target {
	rcs := alloy_relabel.ComponentToPromRelabelConfigs(c.rcs)
	targets := make([]discovery.Target, 0, len(c.prevTargets))
	for _, t := range c.prevTargets {
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
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// check if websocket needs replacement
	if c.websocket != nil && c.websocket.url != newArgs.WebsocketURL {
		if err := c.websocket.Close(); err != nil {
			level.Error(c.logger).Log("msg", "error closing websocket", "err", err)
		}
		c.websocket = nil
		c.websocketStatus = "connecting"
	}

	// check if the websocket connection needs to be established
	if c.websocket == nil {
		req := &http.Request{Header: make(http.Header)}
		if newArgs.WebsocketBasicAuth != nil {
			req.SetBasicAuth(newArgs.WebsocketBasicAuth.Username, string(newArgs.WebsocketBasicAuth.Password))
		}
		w, err := c.newWebSocket(newArgs.WebsocketURL, req.Header)
		if err != nil {
			level.Error(c.logger).Log("msg", "error creating websocket", "err", err)
			return nil
		}
		w.c.SetCloseHandler(c.websocketCloseHandler)
		c.websocket = w
		c.websocketStatus = "connected"
	}

	c.prevTargets = newArgs.Targets
	c.websocket.publishTargets(copyTargets(newArgs.Targets))

	c.opts.OnStateChange(Exports{
		WebsocketStatus: c.websocketStatus,
		Output:          c.underLockFilterTargets(newArgs.Targets),
		Rules:           c.rcs,
	})

	return nil
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

func copyTargets(disc []discovery.Target) []map[string]string {
	t := make([]map[string]string, len(disc))
	for idx, m := range disc {
		n := make(map[string]string, len(m))
		for key, value := range m {
			n[key] = value
		}
		t[idx] = n
	}
	return t
}

func mapToTargets(m []map[string]string) []*settingsv1.CollectionTarget {
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

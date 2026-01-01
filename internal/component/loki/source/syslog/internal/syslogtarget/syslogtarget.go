package syslogtarget

// This code is copied from Promtail v3.1.0 (935aee77ed389c825d36b8d6a85c0d83895a24d1)
// The syslogtarget package is used to configure and run the targets that can
// read syslog entries and forward them to other loki components.

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/loki/pkg/push"
	"github.com/leodido/go-syslog/v4"
	"github.com/leodido/go-syslog/v4/rfc3164"
	"github.com/leodido/go-syslog/v4/rfc5424"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component/common/loki"
	scrapeconfig "github.com/grafana/alloy/internal/component/loki/source/syslog/config"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

var (
	DefaultIdleTimeout      = 120 * time.Second
	DefaultMaxMessageLength = 8192
	DefaultProtocol         = ProtocolTCP
)

// SyslogTarget listens to syslog messages.
// nolint:revive
type SyslogTarget struct {
	metrics       *Metrics
	logger        log.Logger
	handler       loki.EntryHandler
	config        *scrapeconfig.SyslogTargetConfig
	relabelConfig []*relabel.Config

	transport Transport

	messages     chan message
	messagesDone chan struct{}
}

type message struct {
	labels    model.LabelSet
	message   string
	timestamp time.Time
}

// NewSyslogTarget configures a new SyslogTarget.
func NewSyslogTarget(metrics *Metrics, logger log.Logger, handler loki.EntryHandler, relabel []*relabel.Config, config *scrapeconfig.SyslogTargetConfig) (*SyslogTarget, error) {
	t := &SyslogTarget{
		metrics:       metrics,
		logger:        logger,
		handler:       handler,
		config:        config,
		relabelConfig: relabel,
		messagesDone:  make(chan struct{}),
	}

	switch t.transportProtocol() {
	case ProtocolTCP:
		t.transport = NewSyslogTCPTransport(
			config,
			t.handleMessage,
			t.handleMessageError,
			logger,
		)
	case ProtocolUDP:
		t.transport = NewSyslogUDPTransport(
			config,
			t.handleMessage,
			t.handleMessageError,
			logger,
		)
	default:
		return nil, fmt.Errorf("invalid transport protocol. expected 'tcp' or 'udp', got '%s'", t.transportProtocol())
	}

	t.messages = make(chan message)
	go t.messageSender(handler.Chan())

	err := t.transport.Run()
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (t *SyslogTarget) handleMessageError(err error) {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		level.Debug(t.logger).Log("msg", "connection timed out", "err", ne)
		return
	}
	level.Warn(t.logger).Log("msg", "error parsing syslog stream", "err", err)
	t.metrics.syslogParsingErrors.Inc()
}

func (t *SyslogTarget) handleMessageRFC5424(connLabels labels.Labels, msg *rfc5424.SyslogMessage) {
	if msg.Message == nil {
		t.metrics.syslogEmptyMessages.Inc()
		return
	}

	lb := labels.NewBuilder(connLabels)
	if v := msg.SeverityLevel(); v != nil {
		lb.Set("__syslog_message_severity", *v)
	}
	if v := msg.FacilityLevel(); v != nil {
		lb.Set("__syslog_message_facility", *v)
	}
	if v := msg.Hostname; v != nil {
		lb.Set("__syslog_message_hostname", *v)
	}
	if v := msg.Appname; v != nil {
		lb.Set("__syslog_message_app_name", *v)
	}
	if v := msg.ProcID; v != nil {
		lb.Set("__syslog_message_proc_id", *v)
	}
	if v := msg.MsgID; v != nil {
		lb.Set("__syslog_message_msg_id", *v)
	}

	if t.config.LabelStructuredData && msg.StructuredData != nil {
		for id, params := range *msg.StructuredData {
			id = strings.ReplaceAll(id, "@", "_")
			for name, value := range params {
				key := "__syslog_message_sd_" + id + "_" + name
				lb.Set(key, value)
			}
		}
	}

	processed, _ := relabel.Process(lb.Labels(), t.relabelConfig...)

	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	var timestamp time.Time
	if t.config.UseIncomingTimestamp && msg.Timestamp != nil {
		timestamp = *msg.Timestamp
	} else {
		timestamp = time.Now()
	}

	m := *msg.Message
	if t.config.UseRFC5424Message {
		fullMsg, err := msg.String()
		if err != nil {
			level.Debug(t.logger).Log("msg", "failed to convert rfc5424 message to string; using message field instead", "err", err)
		} else {
			m = fullMsg
		}
	}
	t.messages <- message{filtered, m, timestamp}
}

func (t *SyslogTarget) handleMessageRFC3164(connLabels labels.Labels, msg *rfc3164.SyslogMessage) {
	if msg.Message == nil {
		t.metrics.syslogEmptyMessages.Inc()
		return
	}

	lb := labels.NewBuilder(connLabels)
	if v := msg.SeverityLevel(); v != nil {
		lb.Set("__syslog_message_severity", *v)
	}
	if v := msg.FacilityLevel(); v != nil {
		lb.Set("__syslog_message_facility", *v)
	}
	if v := msg.Hostname; v != nil {
		lb.Set("__syslog_message_hostname", *v)
	}
	if v := msg.Appname; v != nil {
		lb.Set("__syslog_message_app_name", *v)
	}
	if v := msg.ProcID; v != nil {
		lb.Set("__syslog_message_proc_id", *v)
	}
	if v := msg.MsgID; v != nil {
		lb.Set("__syslog_message_msg_id", *v)
	}

	// cisco-specific fields
	if v := msg.MessageCounter; v != nil {
		lb.Set("__syslog_message_msg_counter", strconv.Itoa(int(*v)))
	}
	if v := msg.Sequence; v != nil {
		lb.Set("__syslog_message_sequence", strconv.Itoa(int(*v)))
	}

	processed, _ := relabel.Process(lb.Labels(), t.relabelConfig...)

	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	var timestamp time.Time
	if t.config.UseIncomingTimestamp && msg.Timestamp != nil {
		timestamp = *msg.Timestamp
	} else {
		timestamp = time.Now()
	}

	m := *msg.Message

	t.messages <- message{filtered, m, timestamp}
}

func (t *SyslogTarget) handleMessageRaw(connLabels labels.Labels, msg *syslog.Base) {
	if msg.Message == nil || *msg.Message == "" {
		t.metrics.syslogEmptyMessages.Inc()
		return
	}

	lb := labels.NewBuilder(connLabels)
	if v := msg.SeverityLevel(); v != nil {
		lb.Set("__syslog_message_severity", *v)
	}
	if v := msg.FacilityLevel(); v != nil {
		lb.Set("__syslog_message_facility", *v)
	}

	processed, _ := relabel.Process(lb.Labels(), t.relabelConfig...)
	filtered := make(model.LabelSet)
	processed.Range(func(lbl labels.Label) {
		if strings.HasPrefix(lbl.Name, "__") {
			return
		}
		filtered[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	// Timestamp isn't available during raw parse.
	t.messages <- message{
		labels:    filtered,
		message:   *msg.Message,
		timestamp: time.Now(),
	}
}

func (t *SyslogTarget) handleMessage(connLabels labels.Labels, msg syslog.Message) {
	switch m := msg.(type) {
	case *rfc3164.SyslogMessage:
		t.handleMessageRFC3164(connLabels, m)
	case *rfc5424.SyslogMessage:
		t.handleMessageRFC5424(connLabels, m)
	case *syslog.Base:
		t.handleMessageRaw(connLabels, m)
	default:
		level.Error(t.logger).Log("msg", fmt.Sprintf("handleMessage: unsupported message type %T", m))
	}
}

func (t *SyslogTarget) messageSender(entries chan<- loki.Entry) {
	for msg := range t.messages {
		entries <- loki.Entry{
			Labels: msg.labels,
			Entry: push.Entry{
				Timestamp: msg.timestamp,
				Line:      msg.message,
			},
		}
		t.metrics.syslogEntries.Inc()
	}
	t.messagesDone <- struct{}{}
}

// Ready indicates whether or not the syslog target is ready to be read from.
func (t *SyslogTarget) Ready() bool {
	return t.transport.Ready()
}

// Labels returns the set of labels that statically apply to all log entries
// produced by the SyslogTarget.
func (t *SyslogTarget) Labels() model.LabelSet {
	return t.config.Labels
}

// Stop shuts down the SyslogTarget.
func (t *SyslogTarget) Stop() error {
	err := t.transport.Close()
	t.transport.Wait()
	close(t.messages)
	// wait for all pending messages to be processed and sent to handler
	<-t.messagesDone
	t.handler.Stop()
	return err
}

// ListenAddress returns the address SyslogTarget is listening on.
func (t *SyslogTarget) ListenAddress() net.Addr {
	return t.transport.Addr()
}

func (t *SyslogTarget) transportProtocol() string {
	if t.config.ListenProtocol != "" {
		return t.config.ListenProtocol
	}
	return DefaultProtocol
}

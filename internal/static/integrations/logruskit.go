package integrations //nolint:golint

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/go-kit/log/level"

	"github.com/go-kit/log"
	"github.com/sirupsen/logrus"
)

// NewLogger translates log.Logger to logrus.Logger
func NewLogger(logger log.Logger, opts ...Option) *logrus.Logger {
	o := output{}
	for _, apply := range opts {
		apply(&o)
	}

	l := logrus.New()
	l.SetFormatter(formatter{})
	l.SetOutput(output{l: logger})
	return l
}

// Option Exposes logging options
type Option func(*output)

// WithTimestampFromLogrus sets tsFromLogrus logging option
func WithTimestampFromLogrus() Option {
	return func(o *output) {
		o.tsFromLogrus = true
	}
}

type output struct {
	l            log.Logger
	tsFromLogrus bool
}

func (o output) Write(data []byte) (n int, err error) {
	var ll line
	if err := json.Unmarshal(data, &ll); err != nil {
		return 0, fmt.Errorf("can't unmarshal line: %w", err)
	}
	keys := make([]string, 0, len(ll.Data))
	for k := range ll.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	vals := make([]interface{}, 0, 2*len(ll.Data)+2)
	for _, k := range keys {
		vals = append(vals, k, ll.Data[k])
	}
	vals = append(vals, "msg", ll.Message)

	logger := o.l
	if o.tsFromLogrus {
		logger = log.WithPrefix(o.l, "ts", ll.Time)
	}

	lvl := level.InfoValue()
	if mappedLvl, ok := levelMap[ll.Level]; ok {
		lvl = mappedLvl
	}
	logger = log.WithPrefix(logger, level.Key(), lvl)

	return 0, logger.Log(vals...)
}

type formatter struct{}

func (f formatter) Format(e *logrus.Entry) ([]byte, error) {
	ll := line{
		Time:    e.Time,
		Level:   e.Level,
		Data:    e.Data,
		Message: e.Message,
	}

	// FIXME: this can be much better in performance, but we can change it anytime since it's an internal contract
	return json.Marshal(ll)
}

type line struct {
	Time    time.Time
	Level   logrus.Level
	Data    logrus.Fields
	Message string
}

var levelMap = map[logrus.Level]level.Value{
	logrus.PanicLevel: level.ErrorValue(),
	logrus.FatalLevel: level.ErrorValue(),
	logrus.ErrorLevel: level.ErrorValue(),
	logrus.WarnLevel:  level.WarnValue(),
	logrus.InfoLevel:  level.InfoValue(),
	logrus.DebugLevel: level.DebugValue(),
	logrus.TraceLevel: level.DebugValue(),
}

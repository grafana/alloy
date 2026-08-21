package stages

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding"
	"encoding/hex"
	"errors"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"golang.org/x/crypto/sha3"

	"github.com/grafana/alloy/syntax"
)

var errTemplateSourceRequired = errors.New("template source value is required")

var extraFunctionMap = template.FuncMap{
	"ToLower":    strings.ToLower,
	"ToUpper":    strings.ToUpper,
	"Replace":    strings.Replace,
	"Trim":       strings.Trim,
	"TrimLeft":   strings.TrimLeft,
	"TrimRight":  strings.TrimRight,
	"TrimPrefix": strings.TrimPrefix,
	"TrimSuffix": strings.TrimSuffix,
	"TrimSpace":  strings.TrimSpace,
	"Hash": func(salt string, input string) string {
		hash := sha3.Sum256([]byte(salt + input))
		return hex.EncodeToString(hash[:])
	},
	"Sha2Hash": func(salt string, input string) string {
		hash := sha256.Sum256([]byte(salt + input))
		return hex.EncodeToString(hash[:])
	},
	"regexReplaceAll": func(regex string, s string, repl string) string {
		r := regexp.MustCompile(regex)
		return r.ReplaceAllString(s, repl)
	},
	"regexReplaceAllLiteral": func(regex string, s string, repl string) string {
		r := regexp.MustCompile(regex)
		return r.ReplaceAllLiteralString(s, repl)
	},
}

var functionMap = sprig.TxtFuncMap()

func init() {
	for k, v := range extraFunctionMap {
		functionMap[k] = v
	}
}

var _ syntax.Validator = (*TemplateConfig)(nil)

// TemplateConfig configures template value extraction.
type TemplateConfig struct {
	Source   string   `alloy:"source,attr"`
	Template Template `alloy:"template,attr"`
}

func (t *TemplateConfig) Validate() error {
	if t.Source == "" {
		return errTemplateSourceRequired
	}
	return nil
}

var (
	_ encoding.TextMarshaler   = Template("")
	_ encoding.TextUnmarshaler = (*Template)(nil)
)

type Template string

func (t *Template) UnmarshalText(text []byte) error {
	str := Template(text)
	_, err := str.parse()
	if err != nil {
		return err
	}
	*t = str
	return nil
}

func (t Template) MarshalText() (text []byte, err error) {
	return []byte(t), nil
}

func (t Template) parse() (*template.Template, error) {
	return template.New("pipeline_template").Funcs(functionMap).Parse(string(t))
}

var (
	_ Stage = (*templateStage)(nil)
	_ stage = (*templateStage)(nil)
)

// newTemplateStage creates a new templateStage
func newTemplateStage(logger *slog.Logger, config TemplateConfig, next NextFn) (*templateStage, error) {
	templ, err := config.Template.parse()
	// We should not get an error here when built from alloy syntax.
	if err != nil {
		return nil, err
	}
	return &templateStage{
		next:     next,
		cfg:      config,
		template: templ,
		logger:   logger.With("stage", "template"),
	}, nil
}

// templateStage will mutate the incoming entry and set it from extracted data
type templateStage struct {
	next     NextFn
	cfg      TemplateConfig
	template *template.Template
	logger   *slog.Logger
}

// Run implements Stage.
func (o *templateStage) Run(in chan Entry) chan Entry {
	return RunWith(in, func(e Entry) Entry {
		return o.processEntry(e)
	})
}

// process implements stage.
func (o *templateStage) process(ctx context.Context, entries []Entry) error {
	for i := range entries {
		entries[i] = o.processEntry(entries[i])
	}
	return o.next(ctx, entries)
}

// Cleanup implements Stage.
func (o *templateStage) Cleanup() {}

var bufPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func (o *templateStage) processEntry(e Entry) Entry {
	// We allocate space for all extracted values + Value and Entry
	td := make(map[string]any, len(e.Extracted)+2)
	for k, v := range e.Extracted {
		s, err := getString(v)
		if err != nil {
			if debugEnabled(o.logger) {
				o.logger.Debug("extracted template could not be converted to a string", "err", err, "type", reflect.TypeOf(v))
			}
			continue
		}
		td[k] = s
		if k == o.cfg.Source {
			td["Value"] = s
		}
	}
	td["Entry"] = e.Line

	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	err := o.template.Execute(buf, td)
	if err != nil {
		if debugEnabled(o.logger) {
			o.logger.Debug("failed to execute template on extracted value", "err", err)
		}
		return e
	}
	st := buf.String()
	// If the template evaluates to an empty string, remove the key from the map
	if st == "" {
		delete(e.Extracted, o.cfg.Source)
	} else {
		e.Extracted[o.cfg.Source] = st
	}

	return e
}

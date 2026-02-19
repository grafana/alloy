package stages

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha3"
	"encoding/hex"
	"errors"
	"maps"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-kit/log"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Config Errors.
var (
	ErrTemplateSourceRequired = errors.New("template source value is required")
)

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
	maps.Copy(functionMap, extraFunctionMap)
}

// TemplateConfig configures template value extraction.
type TemplateConfig struct {
	Source   string `alloy:"source,attr"`
	Template string `alloy:"template,attr"`
}

// validateTemplateConfig validates the templateStage config.
func validateTemplateConfig(cfg TemplateConfig) (*template.Template, error) {
	if cfg.Source == "" {
		return nil, ErrTemplateSourceRequired
	}

	return template.New("pipeline_template").Funcs(functionMap).Parse(cfg.Template)
}

// newTemplateStage creates a new templateStage
func newTemplateStage(logger log.Logger, config TemplateConfig) (Stage, error) {
	t, err := validateTemplateConfig(config)
	if err != nil {
		return nil, err
	}

	return toStage(&templateStage{
		cfgs:     config,
		logger:   logger,
		template: t,
	}), nil
}

// templateStage will mutate the incoming entry and set it from extracted data
type templateStage struct {
	cfgs     TemplateConfig
	logger   log.Logger
	template *template.Template
}

var bufPool = sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

// Process implements Stage
func (o *templateStage) Process(labels model.LabelSet, extracted map[string]any, t *time.Time, entry *string) {
	// We allocate space for all extracted values + Value and Entry
	td := make(map[string]any, len(extracted)+2)
	for k, v := range extracted {
		s, err := getString(v)
		if err != nil {
			if Debug {
				level.Debug(o.logger).Log("msg", "extracted template could not be converted to a string", "err", err, "type", reflect.TypeOf(v))
			}
			continue
		}
		td[k] = s
		if k == o.cfgs.Source {
			td["Value"] = s
		}
	}
	td["Entry"] = *entry

	buf := bufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		bufPool.Put(buf)
	}()

	err := o.template.Execute(buf, td)
	if err != nil {
		if Debug {
			level.Debug(o.logger).Log("msg", "failed to execute template on extracted value", "err", err)
		}
		return
	}
	st := buf.String()
	// If the template evaluates to an empty string, remove the key from the map
	if st == "" {
		delete(extracted, o.cfgs.Source)
	} else {
		extracted[o.cfgs.Source] = st
	}
}

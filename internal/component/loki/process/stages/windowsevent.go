package stages

import (
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/util/strutil"
)

const (
	defaultWindowsEventSource = "message"
	descriptionLabel          = "Description"
)

type WindowsEventConfig struct {
	Source            string `alloy:"source,attr,optional"`
	DropInvalidLabels bool   `alloy:"drop_invalid_labels,attr,optional"`
	OverwriteExisting bool   `alloy:"overwrite_existing,attr,optional"`
}

func (e *WindowsEventConfig) Validate() error {
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if !model.LabelName(e.Source).IsValid() {
		return fmt.Errorf(ErrInvalidLabelName, e.Source)
	}
	return nil
}

func (e *WindowsEventConfig) SetToDefault() {
	e.Source = defaultWindowsEventSource
}

type WindowsEventStage struct {
	cfg    *WindowsEventConfig
	logger log.Logger

	keyReplacer   *strings.Replacer
	valueReplacer *strings.Replacer
}

// Create a windowsevent stage, including validating any supplied configuration
func newWindowsEventStage(logger log.Logger, cfg *WindowsEventConfig) Stage {
	return &WindowsEventStage{
		cfg:           cfg,
		logger:        log.With(logger, "component", "stage", "type", "windowsevent"),
		keyReplacer:   strings.NewReplacer("\t", "", "\r", "", "\n", "", " ", ""),
		valueReplacer: strings.NewReplacer("\t", "", "\r", "", "\n", ""),
	}
}

func (w *WindowsEventStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	key := w.cfg.Source
	go func() {
		defer close(out)
		for e := range in {
			err := w.processEntry(e.Extracted, key)
			if err != nil {
				continue
			}
			out <- e
		}
	}()
	return out
}

// Process a windows event message from extracted with the specified key, adding additional
// entries into the extracted map.
func (w *WindowsEventStage) processEntry(extracted map[string]any, key string) error {
	value, ok := extracted[key]
	if !ok {
		if Debug {
			level.Debug(w.logger).Log("msg", "source not in the extracted values", "source", key)
		}
		return nil
	}
	s, err := getString(value)
	if err != nil {
		level.Warn(w.logger).Log("msg", "invalid label value parsed", "value", value)
		return err
	}

	// Messages are expected to have sections that are split by empty lines.
	sections := strings.Split(s, "\r\n\r\n")
	for i, section := range sections {
		// The first section is extracted as the description of the message.
		if i == 0 {
			ek, err := w.sanitizeKey(descriptionLabel, extracted)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			ev, err := w.sanitizeValue(section)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			extracted[ek] = ev
			continue
		}

		j := 0
		lines := strings.Split(section, "\r\n")
		keyPrefix := ""
		for j < len(lines) {
			parts := strings.SplitN(lines[j], ":", 2)

			// Skip lines that don't follow the key:value pattern.
			if len(parts) < 2 {
				j++
				continue
			}

			ek := parts[0]
			ev := parts[1]
			j++

			if ev == "" {
				// Some messages have a section title such has:
				// Logon Information:
				// 	Logon Type:5
				//  Virtual Account:No
				// To avoid collisions with other sections, we use the section title as prefix
				if j == 1 {
					// The prefix is not sanitized here because the sanitization process should be
					// applied on the full key only. Else it can add an unnecessary "_extracted" suffix to the prefix.
					keyPrefix = ek
				}
				continue
			}

			// Handle multi-line values.
			// Following lines that are not empty and don't contain a ":" are considered part of the previous value.
			for j < len(lines) && lines[j] != "" && !strings.Contains(lines[j], ":") {
				ev += "," + lines[j]
				j++
			}

			if keyPrefix != "" {
				ek = keyPrefix + "_" + ek
			}

			sanitizedKey, err := w.sanitizeKey(ek, extracted)
			if err != nil {
				w.logParseErr(err)
				continue
			}

			sanitizedValue, err := w.sanitizeValue(ev)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			extracted[sanitizedKey] = sanitizedValue
		}
	}
	if Debug {
		level.Debug(w.logger).Log("msg", "extracted data debug in windowsevent stage",
			"extracted data", fmt.Sprintf("%v", extracted))
	}
	return nil
}

func (w *WindowsEventStage) sanitizeKey(ekey string, extracted map[string]any) (string, error) {
	k := w.keyReplacer.Replace(ekey)
	// TODO: add support for different validation schemes.
	//nolint:staticcheck
	if !model.LabelName(k).IsValid() {
		if w.cfg.DropInvalidLabels {
			return "", fmt.Errorf("invalid label parsed from message, key: %s", k)
		}
		k = strutil.SanitizeFullLabelName(k)
	}
	if _, ok := extracted[k]; ok && !w.cfg.OverwriteExisting {
		level.Info(w.logger).Log("msg", "extracted key that already existed, appending _extracted to key",
			"key", k)
		k += "_extracted"
	}
	return k, nil
}

func (w *WindowsEventStage) sanitizeValue(evalue string) (string, error) {
	v := strings.TrimSpace(w.valueReplacer.Replace(evalue))
	if !model.LabelValue(v).IsValid() {
		return "", fmt.Errorf("invalid value parsed from message, value: %s", v)
	}
	return v, nil
}

func (w *WindowsEventStage) logParseErr(err error) {
	if Debug {
		level.Debug(w.logger).Log("msg", err.Error())
	}
}

func (w *WindowsEventStage) Name() string {
	return StageTypeWindowsEvent
}

// Cleanup implements Stage.
func (*WindowsEventStage) Cleanup() {
	// no-op
}

package stages

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

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
		return fmt.Errorf(errInvalidLabelName, e.Source)
	}
	return nil
}

func (e *WindowsEventConfig) SetToDefault() {
	e.Source = defaultWindowsEventSource
}

var (
	_ Stage          = (*windowsEventStage)(nil)
	_ entryProcessor = (*windowsEventStage)(nil)
)

// Create a windowsevent stage.
func newWindowsEventStage(logger *slog.Logger, cfg *WindowsEventConfig, next NextFn) *windowsEventStage {
	return &windowsEventStage{
		next:          next,
		cfg:           cfg,
		logger:        logger.With("stage", "windowsevent"),
		keyReplacer:   strings.NewReplacer("\t", "", "\r", "", "\n", "", " ", ""),
		valueReplacer: strings.NewReplacer("\t", "", "\r", "", "\n", ""),
	}
}

type windowsEventStage struct {
	next   NextFn
	cfg    *WindowsEventConfig
	logger *slog.Logger

	keyReplacer   *strings.Replacer
	valueReplacer *strings.Replacer
}

func (w *windowsEventStage) Run(in chan Entry) chan Entry {
	out := make(chan Entry)
	go func() {
		defer close(out)
		for e := range in {
			e, err := w.processEntry(e)
			if err != nil {
				continue
			}
			out <- e
		}
	}()
	return out
}

// process implements stage.
func (w *windowsEventStage) process(ctx context.Context, entries []Entry) error {
	var dst int

	for _, e := range entries {
		e, err := w.processEntry(e)
		if err != nil {
			continue
		}
		entries[dst] = e
		dst++
	}

	if dst == 0 {
		return nil
	}

	return w.next(ctx, entries[:dst])
}

// Process a windows event message from extracted with the specified key, adding additional
// entries into the extracted map.
func (w *windowsEventStage) processEntry(e Entry) (Entry, error) {
	value, ok := e.Extracted[w.cfg.Source]
	if !ok {
		if debugEnabled(w.logger) {
			w.logger.Debug("source not in the extracted values", "source", w.cfg.Source)
		}
		return e, nil
	}
	s, err := getString(value)
	if err != nil {
		w.logger.Warn("invalid label value parsed", "value", value)
		return e, err
	}

	// Messages are expected to have sections that are split by empty lines.
	sections := strings.Split(s, "\r\n\r\n")
	for i, section := range sections {
		// The first section is extracted as the description of the message.
		if i == 0 {
			ek, err := w.sanitizeKey(descriptionLabel, e.Extracted)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			ev, err := w.sanitizeValue(section)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			e.Extracted[ek] = ev
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

			sanitizedKey, err := w.sanitizeKey(ek, e.Extracted)
			if err != nil {
				w.logParseErr(err)
				continue
			}

			sanitizedValue, err := w.sanitizeValue(ev)
			if err != nil {
				w.logParseErr(err)
				continue
			}
			e.Extracted[sanitizedKey] = sanitizedValue
		}
	}
	if debugEnabled(w.logger) {
		w.logger.Debug("extracted data debug in windowsevent stage", "extracted_data", e.Extracted)
	}
	return e, nil
}

func (w *windowsEventStage) sanitizeKey(ekey string, extracted map[string]any) (string, error) {
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
		w.logger.Info("extracted key that already existed, appending _extracted to key", "key", k)
		k += "_extracted"
	}
	return k, nil
}

func (w *windowsEventStage) sanitizeValue(evalue string) (string, error) {
	v := strings.TrimSpace(w.valueReplacer.Replace(evalue))
	if !model.LabelValue(v).IsValid() {
		return "", fmt.Errorf("invalid value parsed from message, value: %s", v)
	}
	return v, nil
}

func (w *windowsEventStage) logParseErr(err error) {
	if debugEnabled(w.logger) {
		w.logger.Debug(err.Error())
	}
}

// Cleanup implements Stage.
func (*windowsEventStage) Cleanup() {
	// no-op
}

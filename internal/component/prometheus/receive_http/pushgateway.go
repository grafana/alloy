package receive_http

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/storage"
)

const (
	// pushPathVar is the mux variable holding the grouping labels of a push,
	// which is everything following the /metrics/ prefix of the request path.
	pushPathVar = "labels"

	// base64Suffix marks the label value following a label name in the request
	// path as base64 encoded.
	base64Suffix = "@base64"

	// fallbackContentType is used for pushes that don't declare a supported
	// Content-Type. Pushgateway assumes the Prometheus text format in that case.
	fallbackContentType = "text/plain"
)

// handlePush forwards metrics pushed in a Prometheus exposition format to the
// configured receivers. The request path carries the grouping labels the same
// way the Pushgateway API does, so Prometheus client libraries can push to this
// component without changes.
//
// Unlike Pushgateway, nothing is stored: a push is forwarded as-is, the same
// way a remote write request is.
func (c *Component) handlePush(w http.ResponseWriter, r *http.Request) {
	groupingLabels, err := parseGroupingLabels(mux.Vars(r)[pushPathVar])
	if err != nil {
		c.failPush(w, http.StatusBadRequest, err)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		c.failPush(w, http.StatusBadRequest, fmt.Errorf("failed to read request body: %w", err))
		return
	}

	c.updateMut.RLock()
	appendMetadata := c.args.AppendMetadata
	enableTypeAndUnitLabels := c.args.EnableTypeAndUnitLabels
	c.updateMut.RUnlock()

	// A Content-Type that isn't an exposition format falls back to the text
	// format instead of failing, the same way Pushgateway does, because clients
	// such as `curl --data-binary` send one that says nothing about the body.
	// The error is then only informational, so a nil parser is the fatal case.
	parser, err := textparse.New(body, r.Header.Get("Content-Type"), nil, textparse.ParserOptions{
		EnableTypeAndUnitLabels: enableTypeAndUnitLabels,
		FallbackContentType:     fallbackContentType,
	})
	if parser == nil {
		c.failPush(w, http.StatusUnsupportedMediaType, fmt.Errorf("unsupported Content-Type %q: %v", r.Header.Get("Content-Type"), err))
		return
	}

	app := c.fanout.Appender(r.Context())
	status, err := c.appendExposition(app, parser, groupingLabels, timestamp.FromTime(time.Now()), appendMetadata)
	if err != nil {
		if rollbackErr := app.Rollback(); rollbackErr != nil {
			c.opts.Logger.Warn("failed to roll back pushed metrics", "err", rollbackErr)
		}
		c.failPush(w, status, err)
		return
	}

	if err := app.Commit(); err != nil {
		c.failPush(w, http.StatusInternalServerError, fmt.Errorf("failed to forward pushed metrics: %w", err))
		return
	}
}

// appendExposition appends every sample of an exposition payload to app,
// overriding the labels of each series with groupingLabels. Samples that carry
// no timestamp of their own are stamped with defaultTS. It returns the HTTP
// status to respond with when appending fails.
func (c *Component) appendExposition(
	app storage.Appender,
	parser textparse.Parser,
	groupingLabels labels.Labels,
	defaultTS int64,
	appendMetadata bool,
) (int, error) {

	var (
		lset    labels.Labels
		builder = labels.NewBuilder(labels.EmptyLabels())
		ex      exemplar.Exemplar

		// Help, type, and unit are exposed one at a time and always precede the
		// series of the metric family they describe.
		familyName string
		familyMeta metadata.Metadata
	)

	for {
		entry, err := parser.Next()
		if errors.Is(err, io.EOF) {
			return http.StatusOK, nil
		}
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to parse pushed metrics: %w", err)
		}

		var (
			isHistogram bool
			parsedTS    *int64
			val         float64
			h           *histogram.Histogram
			fh          *histogram.FloatHistogram
		)

		switch entry {
		case textparse.EntrySeries:
			_, parsedTS, val = parser.Series()
		case textparse.EntryHistogram:
			isHistogram = true
			_, parsedTS, h, fh = parser.Histogram()
		case textparse.EntryHelp:
			name, help := parser.Help()
			familyName, familyMeta = trackFamily(familyName, string(name), familyMeta)
			familyMeta.Help = string(help)
			continue
		case textparse.EntryType:
			name, typ := parser.Type()
			familyName, familyMeta = trackFamily(familyName, string(name), familyMeta)
			familyMeta.Type = typ
			continue
		case textparse.EntryUnit:
			name, unit := parser.Unit()
			familyName, familyMeta = trackFamily(familyName, string(name), familyMeta)
			familyMeta.Unit = string(unit)
			continue
		default: // Comments and any entry a future parser may add.
			continue
		}

		parser.Labels(&lset)
		builder.Reset(lset)
		groupingLabels.Range(func(l labels.Label) { builder.Set(l.Name, l.Value) })
		lset = builder.Labels()

		ts := defaultTS
		if parsedTS != nil {
			ts = *parsedTS
		}

		var ref storage.SeriesRef
		if isHistogram {
			ref, err = app.AppendHistogram(0, lset, ts, h, fh)
		} else {
			ref, err = app.Append(0, lset, ts, val)
		}
		if err != nil {
			return http.StatusInternalServerError, fmt.Errorf("failed to append pushed series %s: %w", lset.String(), err)
		}

		for parser.Exemplar(&ex) {
			if !ex.HasTs {
				ex.Ts = ts
			}
			// Exemplars are best effort: a rejected one must not fail the push.
			if _, err := app.AppendExemplar(ref, lset, ex); err != nil {
				c.opts.Logger.Debug("failed to append exemplar of pushed series", "series", lset.String(), "err", err)
			}
			ex = exemplar.Exemplar{}
		}

		if appendMetadata && isSeriesOfFamily(lset, familyName) {
			if _, err := app.UpdateMetadata(ref, lset, familyMeta); err != nil {
				c.opts.Logger.Debug("failed to append metadata of pushed series", "series", lset.String(), "err", err)
			}
		}
	}
}

// failPush responds with an error and logs it. Rejected pushes are logged at
// debug level to keep a misbehaving client from flooding the logs.
func (c *Component) failPush(w http.ResponseWriter, status int, err error) {
	http.Error(w, err.Error(), status)
	if status >= http.StatusInternalServerError {
		c.opts.Logger.Error("failed to ingest pushed metrics", "err", err)
	} else {
		c.opts.Logger.Debug("rejected pushed metrics", "status", status, "err", err)
	}
}

// trackFamily returns the metadata to accumulate the next help, type, or unit
// into, starting over whenever the parser moves on to a new metric family.
func trackFamily(current, next string, meta metadata.Metadata) (string, metadata.Metadata) {
	if current == next {
		return current, meta
	}
	return next, metadata.Metadata{Type: model.MetricTypeUnknown}
}

// isSeriesOfFamily reports whether the metadata of the metric family belongs to
// the series. The suffixed series of a classic histogram or summary, such as
// _bucket or _count, share the metadata of their family.
func isSeriesOfFamily(lset labels.Labels, familyName string) bool {
	if familyName == "" {
		return false
	}
	suffix, ok := strings.CutPrefix(lset.Get(model.MetricNameLabel), familyName)
	return ok && (suffix == "" || strings.HasPrefix(suffix, "_"))
}

// parseGroupingLabels turns the request path following /metrics/ into the
// grouping labels applied to every series of a push. The layout is the one of
// the Pushgateway API, job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}, where a
// label name carrying a @base64 suffix marks its value as base64 encoded.
func parseGroupingLabels(path string) (labels.Labels, error) {
	components := strings.Split(path, "/")
	if len(components) < 2 || len(components)%2 != 0 {
		return labels.EmptyLabels(), fmt.Errorf("invalid path %q: expected /metrics/job/<JOB_NAME>{/<LABEL_NAME>/<LABEL_VALUE>}", path)
	}
	if strings.TrimSuffix(components[0], base64Suffix) != model.JobLabel {
		return labels.EmptyLabels(), fmt.Errorf("invalid path %q: the first label must be %q", path, model.JobLabel)
	}

	builder := labels.NewBuilder(labels.EmptyLabels())
	for i := 0; i < len(components); i += 2 {
		name, isBase64 := strings.CutSuffix(components[i], base64Suffix)
		// Prometheus validates label names as UTF-8 since v3, so the only names
		// left to reject are the ones reserved for internal use.
		if !model.UTF8Validation.IsValidLabelName(name) || strings.HasPrefix(name, model.ReservedLabelPrefix) {
			return labels.EmptyLabels(), fmt.Errorf("invalid label name %q in path %q", name, path)
		}

		value := components[i+1]
		if isBase64 {
			// Padding is optional, and a lone padding character encodes the
			// empty string.
			decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
			if err != nil {
				return labels.EmptyLabels(), fmt.Errorf("invalid base64 value of label %q in path %q: %w", name, path, err)
			}
			value = string(decoded)
		}
		// Setting an empty value drops the label, matching the Prometheus
		// convention that an empty label is an absent one.
		builder.Set(name, value)
	}

	grouping := builder.Labels()
	if grouping.Get(model.JobLabel) == "" {
		return labels.EmptyLabels(), fmt.Errorf("invalid path %q: the job name must not be empty", path)
	}
	return grouping, nil
}

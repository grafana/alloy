package targets

import (
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/graphql/graph/model"
)

type target interface {
	ForEachLabel(func(key string, value string) bool) bool
	HashLabelsWithPredicate(func(key string) bool) uint64
	NonMetaLabelsHash() uint64
}

const maxRuntimeErrorMessageBytes = 1024
const redactedValue = "<redacted>"

func FromComponent(info *component.Info) []model.Target {
	if info == nil {
		return nil
	}
	if provider, ok := info.Component.(component.TargetProvider); ok {
		return fromComponentTargets(provider.ComponentTargets())
	}
	if info.Exports == nil {
		return nil
	}

	var discovered []target
	collectTargets(reflect.ValueOf(info.Exports), &discovered)

	result := make([]model.Target, 0, len(discovered))
	for _, target := range discovered {
		result = append(result, convertTarget(target))
	}
	return result
}

func fromComponentTargets(targets []component.TargetInfo) []model.Target {
	result := make([]model.Target, 0, len(targets))
	for _, target := range targets {
		result = append(result, model.Target{
			Labels:        mapLabelPairs(target.Labels),
			NonMetaLabels: mapLabelPairs(target.NonMetaLabels),
			Hash:          strconv.FormatUint(target.Hash, 10),
			NonMetaHash:   strconv.FormatUint(target.NonMetaHash, 10),
			Scrape:        convertScrapeTarget(target.Scrape),
		})
	}
	return result
}

func convertScrapeTarget(target *component.ScrapeTargetInfo) *model.ScrapeTargetRuntime {
	if target == nil {
		return nil
	}

	result := &model.ScrapeTargetRuntime{
		URL:    redactURL(target.URL),
		Health: scrapeTargetHealth(target.Health),
	}
	if !target.LastAttempt.IsZero() {
		result.LastAttempt = &target.LastAttempt
		duration := model.Duration(target.LastDuration)
		result.LastDuration = &duration
		if target.LastError != nil {
			message := runtimeErrorMessage(target.LastError, target.URL, result.URL)
			result.LastError = &message
		}
	}
	return result
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	return parsed.String()
}

func runtimeErrorMessage(err error, rawURL, redactedURL string) string {
	message := err.Error()
	if rawURL != "" {
		message = strings.ReplaceAll(message, rawURL, redactedURL)
	}
	if len(message) <= maxRuntimeErrorMessageBytes {
		return message
	}

	message = message[:maxRuntimeErrorMessageBytes]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}
	return message
}

func scrapeTargetHealth(health component.ScrapeTargetHealth) model.ScrapeTargetHealth {
	switch health {
	case component.ScrapeTargetHealthUp:
		return model.ScrapeTargetHealthUp
	case component.ScrapeTargetHealthDown:
		return model.ScrapeTargetHealthDown
	default:
		return model.ScrapeTargetHealthUnknown
	}
}

func mapLabelPairs(labels map[string]string) []model.LabelPair {
	result := make([]model.LabelPair, 0, len(labels))
	for name, value := range labels {
		if strings.HasPrefix(name, "__param_") {
			value = redactedValue
		}
		result = append(result, model.LabelPair{Name: name, Value: value})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func collectTargets(value reflect.Value, targets *[]target) {
	if !value.IsValid() {
		return
	}

	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}

	if value.CanInterface() {
		if discoveredTarget, ok := value.Interface().(target); ok {
			*targets = append(*targets, discoveredTarget)
			return
		}
	}

	if value.CanAddr() && value.Addr().CanInterface() {
		if discoveredTarget, ok := value.Addr().Interface().(target); ok {
			*targets = append(*targets, discoveredTarget)
			return
		}
	}

	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if field.CanInterface() {
				collectTargets(field, targets)
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			collectTargets(value.Index(i), targets)
		}
	}
}

func convertTarget(target target) model.Target {
	return model.Target{
		Labels:        labelPairs(target, false),
		NonMetaLabels: labelPairs(target, true),
		Hash: strconv.FormatUint(target.HashLabelsWithPredicate(func(string) bool {
			return true
		}), 10),
		NonMetaHash: strconv.FormatUint(target.NonMetaLabelsHash(), 10),
	}
}

func labelPairs(target target, nonMetaOnly bool) []model.LabelPair {
	var labels []model.LabelPair
	target.ForEachLabel(func(name string, value string) bool {
		if nonMetaOnly && strings.HasPrefix(name, "__meta_") {
			return true
		}
		labels = append(labels, model.LabelPair{Name: name, Value: value})
		return true
	})
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	return labels
}

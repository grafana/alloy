package sdlc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
)

func TestExtractArgumentsUnmarshal(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		extract {
			field {
				key      = "name"
				tag_name = "service"
			}
			label {
				key_regex = "app\\.kubernetes\\.io/(.*)"
				tag_name  = "app.$1"
			}
			label {
				key = "team"
			}
			annotation {
				key      = "kubernetes.io/change-cause"
				tag_name = "change_cause"
			}
		}

		output {}
	`), &args))
	require.Equal(t, ExtractArguments{
		Fields:      []ExtractFieldRule{{Key: "name", TagName: "service"}},
		Labels:      []ExtractRule{{KeyRegex: `app\.kubernetes\.io/(.*)`, TagName: "app.$1"}, {Key: "team"}},
		Annotations: []ExtractRule{{Key: "kubernetes.io/change-cause", TagName: "change_cause"}},
	}, args.Extract)
}

func TestExtractArgumentsValidate(t *testing.T) {
	for name, tc := range map[string]struct {
		args ExtractArguments
		err  string
	}{
		"empty":               {args: ExtractArguments{}},
		"neither key":         {args: ExtractArguments{Labels: []ExtractRule{{TagName: "x"}}}, err: "label: exactly one of key or key_regex must be set"},
		"both keys":           {args: ExtractArguments{Annotations: []ExtractRule{{Key: "a", KeyRegex: "a"}}}, err: "annotation: exactly one of key or key_regex must be set"},
		"invalid regex":       {args: ExtractArguments{Labels: []ExtractRule{{KeyRegex: "("}}}, err: "label: invalid key_regex"},
		"tag without group":   {args: ExtractArguments{Labels: []ExtractRule{{KeyRegex: "app.*", TagName: "app"}}}, err: "requires a capture group"},
		"reference with key":  {args: ExtractArguments{Labels: []ExtractRule{{Key: "app", TagName: "$1"}}}, err: "require key_regex"},
		"duplicate tag names": {args: ExtractArguments{Labels: []ExtractRule{{Key: "a", TagName: "x"}}, Annotations: []ExtractRule{{Key: "b", TagName: "x"}}}, err: `annotation: duplicate tag_name "x"`},
		"unknown field":       {args: ExtractArguments{Fields: []ExtractFieldRule{{Key: "uid"}}}, err: `field: key must be "name" or "namespace"`},
		"missing field key":   {args: ExtractArguments{Fields: []ExtractFieldRule{{TagName: "x"}}}, err: `field: key must be "name" or "namespace"`},
		"field reference":     {args: ExtractArguments{Fields: []ExtractFieldRule{{Key: "name", TagName: "$1"}}}, err: "field: tag_name can't reference capture groups"},
		"duplicate field tag": {args: ExtractArguments{Fields: []ExtractFieldRule{{Key: "name", TagName: "x"}, {Key: "namespace", TagName: "x"}}}, err: `field: duplicate tag_name "x"`},
		"field fallback":      {args: ExtractArguments{Fields: []ExtractFieldRule{{Key: "name", TagName: "x"}}, Labels: []ExtractRule{{Key: "a", TagName: "x"}}}},
		"repeated references": {args: ExtractArguments{Labels: []ExtractRule{{KeyRegex: "a/(.*)", TagName: "$1"}, {KeyRegex: "b/(.*)", TagName: "$1"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			var args Arguments
			args.SetToDefault()
			args.Extract, args.Output = tc.args, &otelcol.ConsumerArguments{}
			err := args.Validate()
			if tc.err == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestExtractorAttributes(t *testing.T) {
	e, err := newExtractor(ExtractArguments{
		Fields: []ExtractFieldRule{
			{Key: "name", TagName: "service"},
			{Key: "namespace"},
		},
		Labels: []ExtractRule{
			// Fields are applied first, so the label wins.
			{Key: "app.kubernetes.io/component", TagName: "service"},
			{KeyRegex: `app\.kubernetes\.io/(?P<field>.*)`, TagName: "app.${field}"},
			{Key: "team"},
			{KeyRegex: "tier"},
			{Key: "missing"},
			{KeyRegex: "empty"},
			{Key: "owner", TagName: "who"},
		},
		Annotations: []ExtractRule{
			{Key: "kubernetes.io/change-cause"},
			{KeyRegex: ".*/last-applied-configuration"},
			{Key: "large"},
			// Labels are applied first, so the annotation wins.
			{Key: "owner-annotation", TagName: "owner"},
		},
	})
	require.NoError(t, err)
	m := &metav1.ObjectMeta{
		Name: "checkout-v2", Namespace: "shop",
		Labels: map[string]string{
			"app.kubernetes.io/component": "api", "app.kubernetes.io/name": "checkout", "app.kubernetes.io/part-of": "shop",
			"team": "payments", "tier": "", "empty": "", "owner": "label", "pod-template-hash": "abc",
		},
		Annotations: map[string]string{
			"kubernetes.io/change-cause": "bump", lastAppliedAnnotation: "{}",
			"large": strings.Repeat("é", maxLabelValueLength), "owner-annotation": "annotation",
		},
	}
	require.Equal(t, map[string]string{
		"service":                    "api",
		"namespace":                  "shop",
		"app.component":              "api",
		"app.name":                   "checkout",
		"app.part-of":                "shop",
		"team":                       "payments",
		"who":                        "label",
		"kubernetes.io/change-cause": "bump",
		"large":                      strings.Repeat("é", maxLabelValueLength/2),
		"owner":                      "annotation",
	}, e.labels(m))
	require.Equal(t, "{}", m.Annotations[lastAppliedAnnotation], "the object isn't modified")

	require.True(t, e.keepAnnotation("kubernetes.io/change-cause"))
	require.False(t, e.keepAnnotation(lastAppliedAnnotation))
	require.False(t, e.keepAnnotation("other"))
	require.Nil(t, extractor{}.labels(m))

	names, err := newExtractor(ExtractArguments{Fields: []ExtractFieldRule{{Key: "name", TagName: "service"}, {Key: "namespace"}}})
	require.NoError(t, err)
	require.Equal(t, map[string]string{"service": "checkout"}, names.labels(&metav1.ObjectMeta{Name: "checkout"}), "empty fields are skipped")
}

func TestExtractedLabelsOnEvents(t *testing.T) {
	c := testController(t)
	var err error
	c.opts.extract, err = newExtractor(ExtractArguments{
		Fields:      []ExtractFieldRule{{Key: "namespace"}},
		Labels:      []ExtractRule{{Key: "team"}},
		Annotations: []ExtractRule{{Key: "owner"}},
	})
	require.NoError(t, err)
	labeled := func(m *metav1.ObjectMeta) {
		m.Labels = map[string]string{"team": "payments"}
		if m.Annotations == nil {
			m.Annotations = map[string]string{}
		}
		m.Annotations["owner"] = "alice"
	}
	want := func(t *testing.T, got decoded) {
		t.Helper()
		require.Equal(t, map[string]any{"namespace": "demo", "team": "payments", "owner": "alice"}, got.attrs["grafana.sdlc.labels"])
	}
	// drain checks every queued event: created, and for Deployments the rollout.
	drain := func(t *testing.T, events int) {
		t.Helper()
		require.Equal(t, events, c.queue.Len())
		for range events {
			event, _ := c.queue.Get()
			c.queue.Done(event)
			want(t, decode(t, event.logs))
		}
	}

	d := deploymentFixture()
	labeled(&d.ObjectMeta)
	c.observe(d, false)
	drain(t, 2)
	s, _ := statefulSetFixture()
	labeled(&s.ObjectMeta)
	c.enqueueCreated(statefulSetWorkload{StatefulSet: s})
	drain(t, 1)
	ds, _ := daemonSetFixture()
	labeled(&ds.ObjectMeta)
	c.enqueueCreated(daemonSetWorkload{DaemonSet: ds})
	drain(t, 1)
	unlabeled, _ := daemonSetFixture()
	unlabeled.Namespace = ""
	c.enqueueCreated(daemonSetWorkload{DaemonSet: unlabeled})
	event, _ := c.queue.Get()
	c.queue.Done(event)
	require.NotContains(t, decode(t, event.logs).attrs, "grafana.sdlc.labels", "omitted when nothing is extracted")

	// Opted-in annotations stay readable in the cache, but the diff still
	// reports only the operation.
	for _, kind := range []configKind{newConfigKind("configmap", contentKeys), newConfigKind("secret", contentMetadata)} {
		t.Run(kind.name, func(t *testing.T) {
			var obj, old any
			transform := configWatchTransform(false, c.opts.extract.keepAnnotation)
			if kind.name == "configmap" {
				cm := configMapFixture()
				old, _ = transform(cm.DeepCopy())
				labeled(&cm.ObjectMeta)
				cm.ResourceVersion = "2"
				obj, _ = transform(cm)
			} else {
				secret := secretFixture()
				old, _ = transform(secret.DeepCopy())
				labeled(&secret.ObjectMeta)
				secret.ResourceVersion = "2"
				obj, _ = transform(secret)
			}
			c.observeConfig(kind, old, obj, false)
			event, got := takeConfig(t, c, "Updated")
			want(t, got)
			require.Equal(t, map[string]any{"operation": "added"}, got.changes()["metadata.annotations.owner"])
			require.NotContains(t, encodedRecord(t, event), fakeSecretValue)
		})
	}
}

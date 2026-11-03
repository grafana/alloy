package sdlc

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/syntax"
)

func testExcluder(t *testing.T, args ...ExcludeArguments) excluder {
	t.Helper()
	e, err := newExcluder(args)
	require.NoError(t, err)
	return e
}

func TestExcludeArgumentsUnmarshal(t *testing.T) {
	var args Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`
		exclude {
			field {
				key   = "metadata.namespace"
				value = "kube-system"
			}
		}
		exclude {
			field {
				key   = "kind"
				value = "Secret"
			}
			label {
				key   = "owner"
				value = "helm"
				op    = "equals"
			}
			annotation {
				key = "sdlc.grafana.com/ignore"
				op  = "exists"
			}
		}

		output {}
	`), &args))
	require.Equal(t, []ExcludeArguments{
		{Fields: []ExcludeRule{{Key: "metadata.namespace", Value: "kube-system"}}},
		{
			Fields:      []ExcludeRule{{Key: "kind", Value: "Secret"}},
			Labels:      []ExcludeRule{{Key: "owner", Value: "helm", Op: "equals"}},
			Annotations: []ExcludeAnnotationRule{{Key: "sdlc.grafana.com/ignore", Op: "exists"}},
		},
	}, args.Exclude)
	require.NoError(t, args.Validate())

	require.ErrorContains(t, syntax.Unmarshal([]byte(`
		exclude {
			label { key = "owner" }
		}
		output {}
	`), &args), `missing required attribute "value"`)
}

func TestExcludeArgumentsValidate(t *testing.T) {
	for name, tc := range map[string]struct {
		args []ExcludeArguments
		err  string
	}{
		"none":              {},
		"empty block":       {args: []ExcludeArguments{{}}, err: "exclude: at least one field, label, or annotation block is required"},
		"unknown field":     {args: []ExcludeArguments{{Fields: []ExcludeRule{{Key: "metadata.uid", Value: "x"}}}}, err: `exclude: field: key must be "metadata.namespace", "metadata.name", or "kind"`},
		"unknown kind":      {args: []ExcludeArguments{{Fields: []ExcludeRule{{Key: "kind", Value: "secret"}}}}, err: "exclude: field: kind must be one of"},
		"kind not equals":   {args: []ExcludeArguments{{Fields: []ExcludeRule{{Key: "kind", Value: "Deployment", Op: "not-equals"}}}}},
		"empty value":       {args: []ExcludeArguments{{Labels: []ExcludeRule{{Key: "owner"}}}}, err: `exclude: label: value is required with op "equals"`},
		"empty key":         {args: []ExcludeArguments{{Labels: []ExcludeRule{{Value: "helm"}}}}, err: "exclude: label: key is required"},
		"unknown op":        {args: []ExcludeArguments{{Labels: []ExcludeRule{{Key: "owner", Value: "helm", Op: "matches"}}}}, err: "exclude: label: op must be one of"},
		"label exists":      {args: []ExcludeArguments{{Labels: []ExcludeRule{{Key: "owner", Value: "helm", Op: "exists"}}}}, err: `exclude: label: op "exists" is only supported for annotations`},
		"field not exists":  {args: []ExcludeArguments{{Fields: []ExcludeRule{{Key: "metadata.name", Value: "x", Op: "does-not-exist"}}}}, err: `exclude: field: op "does-not-exist" is only supported for annotations`},
		"annotation value":  {args: []ExcludeArguments{{Annotations: []ExcludeAnnotationRule{{Key: "a", Value: "x", Op: "exists"}}}}, err: `exclude: annotation: value can't be set with op "exists"`},
		"annotation empty":  {args: []ExcludeArguments{{Annotations: []ExcludeAnnotationRule{{Key: "a"}}}}, err: `exclude: annotation: value is required with op "equals"`},
		"annotation exists": {args: []ExcludeArguments{{Annotations: []ExcludeAnnotationRule{{Key: "a", Op: "exists"}, {Key: "b", Op: "does-not-exist"}}}}},
		"last applied":      {args: []ExcludeArguments{{Annotations: []ExcludeAnnotationRule{{Key: lastAppliedAnnotation, Value: "{}"}}}}, err: "supports only op"},
		"last applied set":  {args: []ExcludeArguments{{Annotations: []ExcludeAnnotationRule{{Key: lastAppliedAnnotation, Op: "exists"}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			var args Arguments
			args.SetToDefault()
			args.Exclude, args.Output = tc.args, &otelcol.ConsumerArguments{}
			err := args.Validate()
			if tc.err == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestExcluderMatches(t *testing.T) {
	m := &metav1.ObjectMeta{Name: "web", Namespace: "kube-system",
		Labels:      map[string]string{"owner": "helm"},
		Annotations: map[string]string{"skip": "true"}}
	bare := &metav1.ObjectMeta{Name: "web", Namespace: "demo"}
	field := func(key, value, op string) ExcludeArguments {
		return ExcludeArguments{Fields: []ExcludeRule{{Key: key, Value: value, Op: op}}}
	}
	label := func(value, op string) ExcludeArguments {
		return ExcludeArguments{Labels: []ExcludeRule{{Key: "owner", Value: value, Op: op}}}
	}
	annotation := func(value, op string) ExcludeArguments {
		return ExcludeArguments{Annotations: []ExcludeAnnotationRule{{Key: "skip", Value: value, Op: op}}}
	}
	for name, tc := range map[string]struct {
		args     []ExcludeArguments
		m        *metav1.ObjectMeta
		excluded bool
	}{
		"no rules":                 {m: m},
		"namespace":                {args: []ExcludeArguments{field("metadata.namespace", "kube-system", "")}, m: m, excluded: true},
		"other namespace":          {args: []ExcludeArguments{field("metadata.namespace", "kube-system", "")}, m: bare},
		"namespace not equals":     {args: []ExcludeArguments{field("metadata.namespace", "kube-system", "not-equals")}, m: bare, excluded: true},
		"name":                     {args: []ExcludeArguments{field("metadata.name", "web", "")}, m: bare, excluded: true},
		"kind":                     {args: []ExcludeArguments{field("kind", "Secret", "")}, m: m, excluded: true},
		"other kind":               {args: []ExcludeArguments{field("kind", "ConfigMap", "")}, m: m},
		"label":                    {args: []ExcludeArguments{label("helm", "")}, m: m, excluded: true},
		"label other value":        {args: []ExcludeArguments{label("argo", "")}, m: m},
		"missing label":            {args: []ExcludeArguments{label("helm", "")}, m: bare},
		"label not equals":         {args: []ExcludeArguments{label("helm", "not-equals")}, m: m},
		"missing label not equals": {args: []ExcludeArguments{label("helm", "not-equals")}, m: bare, excluded: true},
		"annotation":               {args: []ExcludeArguments{annotation("true", "")}, m: m, excluded: true},
		"annotation not equals":    {args: []ExcludeArguments{annotation("true", "not-equals")}, m: m},
		"annotation exists":        {args: []ExcludeArguments{annotation("", "exists")}, m: m, excluded: true},
		"missing annotation":       {args: []ExcludeArguments{annotation("", "exists")}, m: bare},
		"annotation absent":        {args: []ExcludeArguments{annotation("", "does-not-exist")}, m: bare, excluded: true},
		"annotation not absent":    {args: []ExcludeArguments{annotation("", "does-not-exist")}, m: m},
		"all conditions": {args: []ExcludeArguments{{
			Fields: []ExcludeRule{{Key: "kind", Value: "Secret"}},
			Labels: []ExcludeRule{{Key: "owner", Value: "helm"}},
		}}, m: m, excluded: true},
		"one condition fails": {args: []ExcludeArguments{{
			Fields: []ExcludeRule{{Key: "kind", Value: "ConfigMap"}},
			Labels: []ExcludeRule{{Key: "owner", Value: "helm"}},
		}}, m: m},
		"any block": {args: []ExcludeArguments{field("kind", "ConfigMap", ""), label("helm", "")}, m: m, excluded: true},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.excluded, testExcluder(t, tc.args...).excludes("secret", tc.m))
		})
	}
}

func TestExcluderKeepAnnotation(t *testing.T) {
	e := testExcluder(t, ExcludeArguments{Annotations: []ExcludeAnnotationRule{
		{Key: "team", Value: "a"},
		{Key: "env", Value: "dev", Op: "not-equals"},
		{Key: "skip", Op: "exists"},
	}}, ExcludeArguments{Labels: []ExcludeRule{{Key: "owner", Value: "helm"}}})
	require.True(t, e.keepAnnotation("team"))
	require.True(t, e.keepAnnotation("env"))
	require.False(t, e.keepAnnotation("skip"), "presence matches hashed values")
	require.False(t, e.keepAnnotation("owner"))
	require.False(t, excluder{}.keepAnnotation("team"))
}

func TestDeploymentExclusion(t *testing.T) {
	c := testController(t)
	c.opts.exclude = testExcluder(t, ExcludeArguments{Labels: []ExcludeRule{{Key: "sdlc", Value: "off"}}})
	d := deploymentFixture()
	c.observe(d, true)
	revision(d, 2)
	c.observe(d, false)
	take(t, c, "started")
	require.Contains(t, c.collections, string(d.UID))

	// Excluded mid-rollout: the rollout is dropped without superseded or
	// images_resolved events, and later revisions stay silent.
	d.Labels = map[string]string{"sdlc": "off"}
	c.observe(d, false)
	require.Empty(t, c.rollouts)
	require.Empty(t, c.collections)
	revision(d, 3)
	c.observe(d, false)
	succeed(d)
	c.observe(d, false)
	require.Zero(t, c.queue.Len())

	// Back in scope, the current revision is a baseline.
	delete(d.Labels, "sdlc")
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	revision(d, 4)
	c.observe(d, false)
	_, started := take(t, c, "started")
	require.Equal(t, "3", started.attrs["grafana.sdlc.change.version.before"])
	require.Equal(t, true, started.diff["changes_known"])

	// Removing an annotation bumps the generation; the settled rollout isn't
	// mistaken for one in flight.
	c.opts.exclude = testExcluder(t, ExcludeArguments{Annotations: []ExcludeAnnotationRule{{Key: "skip", Op: "exists"}}})
	succeed(d)
	d.Annotations["skip"] = "true"
	c.observe(d, false)
	require.Empty(t, c.rollouts)
	delete(d.Annotations, "skip")
	d.Generation++
	c.observe(d, false)
	require.Empty(t, c.rollouts, "baselined once settled")
	d.Status.ObservedGeneration = d.Generation
	c.observe(d, false)
	require.Zero(t, c.queue.Len())
	require.Equal(t, rolloutSucceeded, c.rollouts[string(d.UID)].status)

	// Deletion forgets the exclusion.
	d.Annotations["skip"] = "true"
	c.observe(d, false)
	require.Contains(t, c.excluded, string(d.UID))
	c.remove(d)
	require.Empty(t, c.excluded)
}

func TestExcludedImagesAreNotReported(t *testing.T) {
	c := testController(t)
	c.opts.exclude = testExcluder(t, ExcludeArguments{Labels: []ExcludeRule{{Key: "sdlc", Value: "off"}}})
	d := deploymentFixture()
	c.observe(d, true)
	revision(d, 2)
	c.observe(d, false)
	take(t, c, "started")
	succeed(d)
	c.observe(d, false)
	take(t, c, "succeeded")
	require.Equal(t, 1, c.imageQueue.Len())

	// Excluded after the rollout closed, before its images were resolved.
	d.Labels = map[string]string{"sdlc": "off"}
	c.observe(d, false)
	collection, _ := c.imageQueue.Get()
	c.finalizeImages(t.Context(), collection)
	c.imageQueue.Done(collection)
	require.Zero(t, c.queue.Len())
}

func TestStatefulSetExclusion(t *testing.T) {
	c := testController(t)
	c.opts.exclude = testExcluder(t, ExcludeArguments{Fields: []ExcludeRule{{Key: "metadata.namespace", Value: "demo"}, {Key: "kind", Value: "StatefulSet"}}})
	s, cr := statefulSetFixture()
	c.observeControllerRevision(cr)
	c.observeRevisioned(s, false, true)
	require.Empty(t, c.workloads)
	updateStatefulSet(s, "web-2")
	c.observeRevisioned(s, false, false)
	c.observeControllerRevision(ownedRevision(s.UID, "StatefulSet", "web-2", 2))
	completeStatefulSet(s)
	c.observeRevisioned(s, false, false)
	require.Zero(t, c.queue.Len())

	// Back in scope, revisions indexed while excluded establish the baseline.
	c.opts.exclude = excluder{}
	c.observeRevisioned(s, true, false)
	require.Zero(t, c.queue.Len(), "a workload returning to scope isn't created")
	require.Equal(t, int64(2), c.rollouts[string(s.UID)].revision)
	updateStatefulSet(s, "web-3")
	c.observeRevisioned(s, false, false)
	c.observeControllerRevision(ownedRevision(s.UID, "StatefulSet", "web-3", 3))
	started := takeStatefulSet(t, c, rolloutStarted)
	require.Equal(t, "2", started.attrs["grafana.sdlc.change.version.before"])

	// A new excluded workload isn't reported as created.
	c.opts.exclude = testExcluder(t, ExcludeArguments{Fields: []ExcludeRule{{Key: "kind", Value: "StatefulSet"}}})
	added, _ := statefulSetFixture()
	added.UID = "s-2"
	c.observeRevisioned(added, true, false)
	require.Zero(t, c.queue.Len())
}

func TestConfigExclusion(t *testing.T) {
	c := testController(t)
	c.opts.exclude = testExcluder(t,
		ExcludeArguments{Fields: []ExcludeRule{{Key: "kind", Value: "ConfigMap"}}, Annotations: []ExcludeAnnotationRule{{Key: "owner", Value: "helm"}}},
		ExcludeArguments{Fields: []ExcludeRule{{Key: "kind", Value: "Secret"}, {Key: "metadata.name", Value: "creds"}}},
	)
	transform := configWatchTransform(false, c.keepAnnotation)
	kind := newConfigKind("configmap", contentMetadata)
	observe := func(old, next *metav1.ObjectMeta) {
		t.Helper()
		var oldObj any
		if old != nil {
			oldObj, _ = transform(&metav1.PartialObjectMetadata{ObjectMeta: *old})
		}
		obj, _ := transform(&metav1.PartialObjectMetadata{ObjectMeta: *next})
		c.observeConfig(kind, oldObj, obj, false)
	}

	// The compared annotation stays readable in the hashed cache.
	helm := configMapFixture().ObjectMeta
	helm.Annotations["owner"] = "helm"
	observe(nil, &helm)
	require.Zero(t, c.queue.Len(), "excluded creation")
	updated := *helm.DeepCopy()
	updated.ResourceVersion = "2"
	updated.Labels["team"] = "b"
	observe(&helm, &updated)
	require.Zero(t, c.queue.Len(), "excluded update")

	// Leaving the scope of exclude rules is a baseline; later updates report.
	released := *updated.DeepCopy()
	released.ResourceVersion = "3"
	delete(released.Annotations, "owner")
	observe(&updated, &released)
	require.Zero(t, c.queue.Len())
	next := *released.DeepCopy()
	next.ResourceVersion = "4"
	next.Labels["team"] = "c"
	observe(&released, &next)
	_, got := takeConfig(t, c, "Updated")
	require.Equal(t, map[string]any{"operation": "modified", "before": "b", "after": "c"}, got.changes()["metadata.labels.team"])

	// Rules apply to the kind they name.
	secrets := newConfigKind("secret", contentMetadata)
	c.observeConfig(secrets, nil, transformed(t, false, secretFixture()), false)
	require.Zero(t, c.queue.Len())
	other := secretFixture()
	other.Name, other.UID = "other", "s-2"
	other.Annotations = map[string]string{"owner": "helm"}
	c.observeConfig(secrets, nil, transformed(t, false, other), false)
	takeConfig(t, c, "Created")
}

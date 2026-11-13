package sdlc

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metadatafake "k8s.io/client-go/metadata/fake"
)

const fakeSecretValue = "hunter2-do-not-export"

func configMapFixture() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "demo", UID: "cm-1", ResourceVersion: "1",
			Labels:        map[string]string{"team": "a"},
			Annotations:   map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"data":{"TOKEN":"` + fakeSecretValue + `"}}`},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}}},
		Data:       map[string]string{"LOG_LEVEL": "info", "TOKEN": fakeSecretValue},
		BinaryData: map[string][]byte{"cert.der": {1, 2, 3}},
	}
}

func secretFixture() *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: "creds", Namespace: "demo", UID: "s-1", ResourceVersion: "1",
			Annotations: map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"data":{"password":"` + fakeSecretValue + `"}}`}},
	}
}

func transformed(t *testing.T, values bool, obj any) any {
	t.Helper()
	out, err := configWatchTransform(values, extractor{}.keepAnnotation)(obj)
	require.NoError(t, err)
	return out
}

func takeConfig(t *testing.T, c *controller, summary string) (*eventBatch, decoded) {
	t.Helper()
	event, got := nextEvent(t, c)
	require.Equal(t, summary, got.summary)
	require.Equal(t, strings.ToLower(summary), got.resource["grafana.sdlc.provider.resource.change.type"])
	require.Equal(t, "effect", got.attrs["grafana.sdlc.type"])
	require.True(t, got.record.TraceID().IsEmpty(), "config changes aren't traced")
	require.Nil(t, got.context, "config changes have no context")
	if summary == "Updated" {
		require.Equal(t, true, got.diff["before_known"])
	} else {
		require.NotContains(t, got.diff, "before_known")
		require.NotContains(t, got.diff, "before")
	}
	if got.diff["snapshots_omitted"] == nil {
		require.Contains(t, got.diff, "after")
	}
	return event, got
}

func encodedRecord(t *testing.T, event *eventBatch) string {
	t.Helper()
	wire, err := (&plog.JSONMarshaler{}).MarshalLogs(event.logs)
	require.NoError(t, err)
	return string(wire)
}

func TestConfigMapChangesByContentMode(t *testing.T) {
	for _, content := range []string{contentAll, contentKeys, contentMetadata} {
		t.Run(content, func(t *testing.T) {
			c := testController(t)
			kind := newConfigKind(configMapKind, content)
			old := configMapFixture()
			next := old.DeepCopy()
			next.ResourceVersion = "2"
			next.Labels["team"] = "b"
			next.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = `{"data":{"TOKEN":"rotated-` + fakeSecretValue + `"}}`
			next.Annotations["owner"] = "b"
			next.Data["LOG_LEVEL"] = "debug"
			next.Data["TOKEN"] = "rotated-" + fakeSecretValue
			delete(next.BinaryData, "cert.der")
			next.Data["NEW"] = "x"
			toObj := func(cm *corev1.ConfigMap) any {
				if content == contentMetadata {
					return transformed(t, false, &metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				}
				return transformed(t, content == contentAll, cm)
			}
			c.observeConfig(kind, toObj(old), toObj(next), false)
			event, got := takeConfig(t, c, "Updated")
			require.Equal(t, map[string]any{
				"grafana.sdlc.provider.type":                 "k8s",
				"grafana.sdlc.provider.id":                   "cluster-1",
				"grafana.sdlc.provider.resource.type":        "configmap",
				"grafana.sdlc.provider.resource.id":          "demo/app",
				"grafana.sdlc.provider.resource.change.type": "updated",
			}, got.resource)
			require.Equal(t, "1", got.attrs["grafana.sdlc.change.version.before"])
			require.Equal(t, "2", got.attrs["grafana.sdlc.change.version.after"])
			wire := encodedRecord(t, event)
			before, _ := got.diff["before"].(map[string]any)
			after, _ := got.diff["after"].(map[string]any)
			require.Equal(t, map[string]any{"team": "a"}, before["metadata"].(map[string]any)["labels"])
			require.Equal(t, map[string]any{"team": "b"}, after["metadata"].(map[string]any)["labels"])
			require.Equal(t, map[string]any{"owner": "b"}, after["metadata"].(map[string]any)["annotations"], "ignored annotations are left out")
			require.NotContains(t, after["metadata"], "managedFields")
			require.Equal(t, "ConfigMap", after["kind"])
			require.Equal(t, "v1", after["apiVersion"])
			for field, want := range map[string]any{"name": "app", "namespace": "demo", "uid": "cm-1", "resourceVersion": "2"} {
				require.Equal(t, want, after["metadata"].(map[string]any)[field], field)
			}
			require.Equal(t, "1", before["metadata"].(map[string]any)["resourceVersion"])
			switch content {
			case contentAll:
				require.Equal(t, map[string]any{"LOG_LEVEL": "info", "TOKEN": fakeSecretValue}, before["data"])
				require.Equal(t, map[string]any{"LOG_LEVEL": "debug", "TOKEN": "rotated-" + fakeSecretValue, "NEW": "x"}, after["data"])
				require.Equal(t, map[string]any{"cert.der": "AQID"}, before["binaryData"])
				require.NotContains(t, after, "binaryData")
			case contentKeys:
				requireRedacted(t, before["data"], "LOG_LEVEL", "TOKEN")
				requireRedacted(t, after["data"], "LOG_LEVEL", "TOKEN", "NEW")
				requireRedacted(t, before["binaryData"], "cert.der")
				// Placeholders show which values changed.
				for _, key := range []string{"LOG_LEVEL", "TOKEN"} {
					require.NotEqual(t, before["data"].(map[string]any)[key], after["data"].(map[string]any)[key], key)
				}
				require.NotContains(t, after, "binaryData")
				require.NotContains(t, wire, fakeSecretValue)
				require.NotContains(t, wire, "debug")
				require.NotContains(t, wire, fingerprintPrefix)
			case contentMetadata:
				require.NotContains(t, before, "data")
				require.NotContains(t, after, "data")
				require.NotContains(t, wire, fakeSecretValue)
			}
		})
	}
}

func TestConfigBaselineCreatedAndUnchanged(t *testing.T) {
	c := testController(t)
	kind := newConfigKind(configMapKind, contentKeys)
	cm := transformed(t, false, configMapFixture())
	c.observeConfig(kind, nil, cm, true)
	require.Zero(t, c.queue.Len(), "initial listing is a baseline")
	c.observeConfig(kind, nil, cm, false)
	_, created := takeConfig(t, c, "Created")
	require.NotContains(t, created.attrs, "grafana.sdlc.change.version.before")
	require.Equal(t, "1", created.attrs["grafana.sdlc.change.version.after"])
	after := created.diff["after"].(map[string]any)
	requireRedacted(t, after["data"], "LOG_LEVEL", "TOKEN")
	requireRedacted(t, after["binaryData"], "cert.der")
	require.Equal(t, map[string]any{"team": "a"}, created.meta("after")["labels"])
	// Relists replay unchanged objects as updates.
	c.observeConfig(kind, cm, cm, false)
	require.Zero(t, c.queue.Len())
}

func TestSecretMetadataOnlyAndInferredDataChange(t *testing.T) {
	c := testController(t)
	kind := newConfigKind(secretKind, contentMetadata)
	bare := secretFixture()
	bare.Annotations = nil
	c.observeConfig(kind, nil, transformed(t, false, bare), false)
	_, created := takeConfig(t, c, "Created")
	require.Equal(t, "secret", created.resource["grafana.sdlc.provider.resource.type"])
	require.Equal(t, map[string]any{"after": map[string]any{"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": "creds", "namespace": "demo", "uid": "s-1", "resourceVersion": "1"}}},
		created.diff, "creation is reported without tracked fields")

	old := transformed(t, false, secretFixture())
	next := secretFixture()
	next.ResourceVersion = "2"
	c.observeConfig(kind, old, transformed(t, false, next), false)
	// A new resource version without a metadata change is reported, although
	// the snapshots differ only in their version.
	_, updated := takeConfig(t, c, "Updated")
	require.Equal(t, "1", updated.meta("before")["resourceVersion"])
	require.Equal(t, "2", updated.meta("after")["resourceVersion"])

	labeled := secretFixture()
	labeled.ResourceVersion = "3"
	labeled.Labels = map[string]string{"rotated": "true"}
	labeled.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = `{"data":{"password":"new-` + fakeSecretValue + `"}}`
	c.observeConfig(kind, transformed(t, false, next), transformed(t, false, labeled), false)
	event, updated := takeConfig(t, c, "Updated")
	require.NotContains(t, updated.meta("before"), "labels")
	require.Equal(t, map[string]any{"rotated": "true"}, updated.meta("after")["labels"])
	require.NotContains(t, encodedRecord(t, event), fakeSecretValue)

	// Client-side kubectl apply changes the data and last-applied-configuration
	// together.
	reapplied := labeled.DeepCopy()
	reapplied.ResourceVersion = "4"
	reapplied.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = `{"data":{"password":"newer-` + fakeSecretValue + `"}}`
	c.observeConfig(kind, transformed(t, false, labeled), transformed(t, false, reapplied), false)
	event, updated = takeConfig(t, c, "Updated")
	require.Equal(t, "4", updated.meta("after")["resourceVersion"])
	require.NotContains(t, encodedRecord(t, event), "newer-")

	// Leader election explains a new resource version.
	leader := reapplied.DeepCopy()
	leader.ResourceVersion = "5"
	leader.Annotations[leaderAnnotation] = `{"holderIdentity":"a"}`
	c.observeConfig(kind, transformed(t, false, reapplied), transformed(t, false, leader), false)
	renewed := leader.DeepCopy()
	renewed.ResourceVersion = "6"
	renewed.Annotations[leaderAnnotation] = `{"holderIdentity":"b"}`
	c.observeConfig(kind, transformed(t, false, leader), transformed(t, false, renewed), false)
	require.Zero(t, c.queue.Len())
}

func TestConfigEmptyValues(t *testing.T) {
	c := testController(t)
	kind := newConfigKind(configMapKind, contentAll)
	old := transformed(t, true, configMapFixture())
	// Missing and empty values are the same state.
	empty := configMapFixture()
	empty.ResourceVersion = "2"
	empty.Data["EMPTY"] = ""
	empty.Annotations["owner"] = ""
	c.observeConfig(kind, old, transformed(t, true, empty), false)
	require.Zero(t, c.queue.Len())
}

func TestConfigWatchTransform(t *testing.T) {
	out := transformed(t, false, configMapFixture()).(*corev1.ConfigMap)
	require.Empty(t, out.ManagedFields)
	require.True(t, strings.HasPrefix(out.Data["TOKEN"], fingerprintPrefix))
	require.True(t, strings.HasPrefix(string(out.BinaryData["cert.der"]), fingerprintPrefix))
	require.NotContains(t, out.Annotations["kubectl.kubernetes.io/last-applied-configuration"], fakeSecretValue)
	require.Equal(t, out, transformed(t, false, out), "list pages and the informer both transform")
	kept := transformed(t, true, configMapFixture()).(*corev1.ConfigMap)
	require.Equal(t, fakeSecretValue, kept.Data["TOKEN"])
	require.Empty(t, kept.ManagedFields)
}

func TestConfigOversizedSnapshotsAreOmitted(t *testing.T) {
	c := testController(t)
	kind := newConfigKind(configMapKind, contentAll)
	old := configMapFixture()
	next := old.DeepCopy()
	next.ResourceVersion = "2"
	next.Labels["team"] = "b"
	next.Data["LARGE"] = strings.Repeat("\"", maxEventBytes/2)
	old.Data["LARGE"] = strings.Repeat("'", maxEventBytes/2)
	c.observeConfig(kind, transformed(t, true, old), transformed(t, true, next), false)
	event, got := takeConfig(t, c, "Updated")
	require.Equal(t, map[string]any{"before_known": true, "snapshots_omitted": "size"}, got.diff)
	size, err := wireSize(event.logs)
	require.NoError(t, err)
	require.LessOrEqual(t, size, maxEventBytes)
}

func TestConfigArgumentsValidate(t *testing.T) {
	args := ConfigMapsArguments{}
	args.SetToDefault()
	require.NoError(t, args.Validate())
	args.Content = "values"
	require.ErrorContains(t, args.Validate(), `content must be one of "all", "keys", or "metadata"`)
}

func TestWatcherTracksConfigMapsAndSecrets(t *testing.T) {
	client := fake.NewClientset(configMapFixture())
	scheme := metadatafake.NewTestScheme()
	require.NoError(t, metav1.AddMetaToScheme(scheme))
	metadataClient := metadatafake.NewSimpleMetadataClient(scheme, secretFixture())
	received := runWatcher(t, controllerOptions{clusterUID: "explicit-cluster", client: client, metadata: metadataClient, configMapContent: contentKeys, secrets: true})
	requireWatching(t, client.Actions, "configmaps")
	requireWatching(t, metadataClient.Actions, "secrets")
	require.Empty(t, received, "baseline objects are silent")

	cm := configMapFixture()
	cm.ResourceVersion = "2"
	cm.Data["LOG_LEVEL"] = "debug"
	_, err := client.CoreV1().ConfigMaps("demo").Update(t.Context(), cm, metav1.UpdateOptions{})
	require.NoError(t, err)
	got := nextReceived(t, received)
	require.Equal(t, "configmap", got.resource["grafana.sdlc.provider.resource.type"])
	require.Equal(t, "1", got.meta("before")["resourceVersion"])
	require.Equal(t, "2", got.meta("after")["resourceVersion"])

	secret := secretFixture()
	secret.Name, secret.UID, secret.ResourceVersion = "new", "s-2", "5"
	require.NoError(t, metadataClient.Tracker().Create(corev1.SchemeGroupVersion.WithResource("secrets"), secret, "demo"))
	got = nextReceived(t, received)
	require.Equal(t, "demo/new", got.resource["grafana.sdlc.provider.resource.id"])
	require.Equal(t, "Created", got.summary)

	require.NoError(t, client.CoreV1().ConfigMaps("demo").Delete(t.Context(), "app", metav1.DeleteOptions{}))
	select {
	case logs := <-received:
		t.Fatalf("unexpected event on delete: %s", decode(t, logs).summary)
	case <-time.After(200 * time.Millisecond):
	}
}

// requireRedacted asserts that values holds exactly keys, each with a redacted
// placeholder.
func requireRedacted(t *testing.T, values any, keys ...string) {
	t.Helper()
	m, ok := values.(map[string]any)
	require.True(t, ok, "%v", values)
	require.Len(t, m, len(keys))
	for _, key := range keys {
		requirePlaceholder(t, m[key], key)
	}
}

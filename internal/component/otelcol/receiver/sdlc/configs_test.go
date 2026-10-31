package sdlc

import (
	"context"
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
	out, err := configWatchTransform(values)(obj)
	require.NoError(t, err)
	return out
}

func takeConfig(t *testing.T, c *controller, summary string) (*eventBatch, decoded) {
	t.Helper()
	require.Positive(t, c.queue.Len())
	event, _ := c.queue.Get()
	c.queue.Done(event)
	got := decode(t, event.logs)
	require.Equal(t, summary, got.summary)
	require.Equal(t, strings.ToLower(summary), got.resource["grafana.sdlc.provider.resource.change.type"])
	require.Equal(t, "UNSET", got.attrs["grafana.sdlc.status"])
	require.Equal(t, "effect", got.attrs["grafana.sdlc.type"])
	require.Equal(t, event.id, got.attrs["grafana.sdlc.event.id"])
	require.True(t, got.record.TraceID().IsEmpty(), "config changes aren't traced")
	if got.diff != nil {
		require.Len(t, got.diff, 1)
		require.NotEmpty(t, got.changes())
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
			kind := newConfigKind("configmap", content)
			old := configMapFixture()
			next := old.DeepCopy()
			next.ResourceVersion = "2"
			next.Labels["team"] = "b"
			next.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = `{"data":{"TOKEN":"rotated-` + fakeSecretValue + `"}}`
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
			changes := got.changes()
			require.Equal(t, map[string]any{"operation": "modified", "before": "a", "after": "b"}, changes["metadata.labels.team"])
			require.Equal(t, map[string]any{"operation": "modified"}, changes["metadata.annotations.kubectl.kubernetes.io/last-applied-configuration"])
			wire := encodedRecord(t, event)
			switch content {
			case contentAll:
				require.Equal(t, map[string]any{"operation": "modified", "before": "info", "after": "debug"}, changes["data.LOG_LEVEL"])
				require.Equal(t, map[string]any{"operation": "added", "after": "x"}, changes["data.NEW"])
				require.Equal(t, map[string]any{"operation": "removed", "before": "AQID"}, changes["binaryData.cert.der"])
			case contentKeys:
				require.Equal(t, map[string]any{"operation": "modified"}, changes["data.LOG_LEVEL"])
				require.Equal(t, map[string]any{"operation": "added"}, changes["data.NEW"])
				require.Equal(t, map[string]any{"operation": "removed"}, changes["binaryData.cert.der"])
				require.NotContains(t, wire, fakeSecretValue)
				require.NotContains(t, wire, "debug")
				require.NotContains(t, wire, redactedPrefix)
			case contentMetadata:
				require.NotContains(t, changes, "data.LOG_LEVEL")
				require.NotContains(t, wire, fakeSecretValue)
			}
			require.Len(t, changes, map[string]int{contentAll: 6, contentKeys: 6, contentMetadata: 2}[content])
		})
	}
}

func TestConfigBaselineCreatedAndUnchanged(t *testing.T) {
	c := testController(t)
	kind := newConfigKind("configmap", contentKeys)
	cm := transformed(t, false, configMapFixture())
	c.observeConfig(kind, nil, cm, true)
	require.Zero(t, c.queue.Len(), "initial listing is a baseline")
	c.observeConfig(kind, nil, cm, false)
	_, created := takeConfig(t, c, "Created")
	require.NotContains(t, created.attrs, "grafana.sdlc.change.version.before")
	require.Equal(t, "1", created.attrs["grafana.sdlc.change.version.after"])
	require.Equal(t, map[string]any{
		"data.LOG_LEVEL":       map[string]any{"operation": "added"},
		"data.TOKEN":           map[string]any{"operation": "added"},
		"binaryData.cert.der":  map[string]any{"operation": "added"},
		"metadata.labels.team": map[string]any{"operation": "added", "after": "a"},
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration": map[string]any{"operation": "added"},
	}, created.changes())
	// Relists replay unchanged objects as updates.
	c.observeConfig(kind, cm, cm, false)
	require.Zero(t, c.queue.Len())
}

func TestSecretMetadataOnlyAndInferredDataChange(t *testing.T) {
	c := testController(t)
	kind := newConfigKind("secret", contentMetadata)
	bare := secretFixture()
	bare.Annotations = nil
	c.observeConfig(kind, nil, transformed(t, false, bare), false)
	_, created := takeConfig(t, c, "Created")
	require.Equal(t, "secret", created.resource["grafana.sdlc.provider.resource.type"])
	require.NotContains(t, created.attrs, "grafana.sdlc.change.diff", "creation is reported without tracked fields")

	old := transformed(t, false, secretFixture())
	next := secretFixture()
	next.ResourceVersion = "2"
	c.observeConfig(kind, old, transformed(t, false, next), false)
	_, updated := takeConfig(t, c, "Updated")
	require.Equal(t, map[string]any{"data": map[string]any{"operation": "modified"}}, updated.changes())

	labeled := secretFixture()
	labeled.ResourceVersion = "3"
	labeled.Labels = map[string]string{"rotated": "true"}
	labeled.Annotations["kubectl.kubernetes.io/last-applied-configuration"] = `{"data":{"password":"new-` + fakeSecretValue + `"}}`
	c.observeConfig(kind, transformed(t, false, next), transformed(t, false, labeled), false)
	event, updated := takeConfig(t, c, "Updated")
	require.Len(t, updated.changes(), 2, "metadata changes suppress the inferred data change")
	require.Equal(t, map[string]any{"operation": "added", "after": "true"}, updated.changes()["metadata.labels.rotated"])
	require.NotContains(t, encodedRecord(t, event), fakeSecretValue)
}

func TestConfigWatchTransform(t *testing.T) {
	out := transformed(t, false, configMapFixture()).(*corev1.ConfigMap)
	require.Empty(t, out.ManagedFields)
	require.True(t, strings.HasPrefix(out.Data["TOKEN"], redactedPrefix))
	require.True(t, strings.HasPrefix(string(out.BinaryData["cert.der"]), redactedPrefix))
	require.NotContains(t, out.Annotations["kubectl.kubernetes.io/last-applied-configuration"], fakeSecretValue)
	require.Equal(t, out, transformed(t, false, out), "list pages and the informer both transform")
	kept := transformed(t, true, configMapFixture()).(*corev1.ConfigMap)
	require.Equal(t, fakeSecretValue, kept.Data["TOKEN"])
	require.Empty(t, kept.ManagedFields)
}

func TestConfigOversizedValuesAreOmitted(t *testing.T) {
	c := testController(t)
	kind := newConfigKind("configmap", contentAll)
	old := configMapFixture()
	next := old.DeepCopy()
	next.ResourceVersion = "2"
	next.Labels["team"] = "b"
	next.Data["LARGE"] = strings.Repeat("\"", maxEventBytes/2)
	old.Data["LARGE"] = strings.Repeat("'", maxEventBytes/2)
	c.observeConfig(kind, transformed(t, true, old), transformed(t, true, next), false)
	event, got := takeConfig(t, c, "Updated")
	require.Equal(t, map[string]any{"operation": "modified"}, got.changes()["data.LARGE"])
	require.Equal(t, map[string]any{"operation": "modified", "before": "a", "after": "b"}, got.changes()["metadata.labels.team"], "label values are kept")
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
	received := make(chan plog.Logs, 8)
	c := newController(controllerOptions{clusterUID: "explicit-cluster", client: client, metadata: metadataClient, configMapContent: contentKeys, secrets: true,
		emit: func(_ context.Context, batch func() eventBatch) error { received <- batch().logs; return nil }})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.run(ctx) }()
	watching := func(actions func() []string, resource string) func() bool {
		return func() bool {
			for _, action := range actions() {
				if action == "watch/"+resource {
					return true
				}
			}
			return false
		}
	}
	clientActions := func() []string {
		out := []string{}
		for _, action := range client.Actions() {
			out = append(out, action.GetVerb()+"/"+action.GetResource().Resource)
		}
		return out
	}
	metadataActions := func() []string {
		out := []string{}
		for _, action := range metadataClient.Actions() {
			out = append(out, action.GetVerb()+"/"+action.GetResource().Resource)
		}
		return out
	}
	require.Eventually(t, watching(clientActions, "configmaps"), time.Second, 10*time.Millisecond)
	require.Eventually(t, watching(metadataActions, "secrets"), time.Second, 10*time.Millisecond)
	require.Empty(t, received, "baseline objects are silent")
	next := func() decoded {
		t.Helper()
		select {
		case logs := <-received:
			return decode(t, logs)
		case <-time.After(3 * time.Second):
			t.Fatal("no event")
			return decoded{}
		}
	}

	cm := configMapFixture()
	cm.ResourceVersion = "2"
	cm.Data["LOG_LEVEL"] = "debug"
	_, err := client.CoreV1().ConfigMaps("demo").Update(ctx, cm, metav1.UpdateOptions{})
	require.NoError(t, err)
	got := next()
	require.Equal(t, "configmap", got.resource["grafana.sdlc.provider.resource.type"])
	require.Equal(t, map[string]any{"data.LOG_LEVEL": map[string]any{"operation": "modified"}}, got.changes())

	secret := secretFixture()
	secret.Name, secret.UID, secret.ResourceVersion = "new", "s-2", "5"
	require.NoError(t, metadataClient.Tracker().Create(corev1.SchemeGroupVersion.WithResource("secrets"), secret, "demo"))
	got = next()
	require.Equal(t, "demo/new", got.resource["grafana.sdlc.provider.resource.id"])
	require.Equal(t, "Created", got.summary)

	require.NoError(t, client.CoreV1().ConfigMaps("demo").Delete(ctx, "app", metav1.DeleteOptions{}))
	select {
	case logs := <-received:
		t.Fatalf("unexpected event on delete: %s", decode(t, logs).summary)
	case <-time.After(200 * time.Millisecond):
	}
	cancel()
	require.NoError(t, <-done)
}

package secret

import (
	"testing"

	"github.com/grafana/alloy/internal/component/remote/kubernetes"
	"github.com/grafana/alloy/internal/component/remote/kubernetes/internal/test_util"
	"github.com/grafana/alloy/syntax/alloytypes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test(t *testing.T) {
	tester := test_util.Tester{
		T:             t,
		ComponentName: "remote.kubernetes.secret",
		ComponentCfg: `
			namespace = "testNamespace"
			name = "testSecretName"`,
		KubeObjects: []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testSecretName",
					Namespace: "testNamespace",
					Labels: map[string]string{
						"label1": "value1",
					},
				},
				Data: map[string][]byte{
					"test.txt": []byte("random json"),
				},
			},
		},
		Expected: kubernetes.Exports{
			Data: map[string]alloytypes.OptionalSecret{
				"test.txt": {
					IsSecret: true,
					Value:    "random json",
				},
			},
		},
	}

	tester.Test()
}

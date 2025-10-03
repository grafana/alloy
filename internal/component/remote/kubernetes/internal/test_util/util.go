package test_util

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/go-kit/log"
	kubernetes_common "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/remote/kubernetes"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	client_go "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type FakeClientBuidler struct {
	KubeObjects []runtime.Object
}

func (b FakeClientBuidler) GetKubernetesClient(_ log.Logger,
	_ *kubernetes_common.ClientArguments) (client_go.Interface, error) {

	fakeClientset := fake.NewClientset(b.KubeObjects...)

	return fakeClientset, nil
}

func Eventually(t *testing.T, min, max time.Duration, retries int, f func() error) {
	t.Helper()

	l := util.TestLogger(t)

	bo := backoff.New(t.Context(), backoff.Config{
		MinBackoff: min,
		MaxBackoff: max,
		MaxRetries: retries,
	})
	for bo.Ongoing() {
		err := f()
		if err == nil {
			return
		}

		level.Error(l).Log("msg", "condition failed", "err", err)
		bo.Wait()
		continue
	}

	require.NoError(t, bo.Err(), "condition failed")
}

type Tester struct {
	T             *testing.T
	ComponentName string
	ComponentCfg  string
	KubeObjects   []runtime.Object
	Expected      kubernetes.Exports
}

func (t *Tester) Test() {
	ctx := componenttest.TestContext(t.T)

	ctrl, err := componenttest.NewControllerFromID(util.TestLogger(t.T), t.ComponentName)
	require.NoError(t.T, err)

	var args kubernetes.Arguments
	require.NoError(t.T, syntax.Unmarshal([]byte(t.ComponentCfg), &args))

	fakeClientBuidler := FakeClientBuidler{
		KubeObjects: t.KubeObjects,
	}

	go func() {
		component, cleanup, err := ctrl.BuildWithoutRun(ctx, args)
		require.NoError(t.T, err)
		defer cleanup()

		cfgMapComponent := component.(*kubernetes.Component)
		cfgMapComponent.ClientBuidler = &fakeClientBuidler

		err = component.Run(ctx)
		require.NoError(t.T, err)
	}()

	require.NoError(t.T, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t.T, ctrl.WaitExports(time.Second), "component never exported anything")

	requireExports := func(expect kubernetes.Exports) {
		Eventually(t.T, 10*time.Millisecond, 10*time.Second, 5, func() error {
			actual := ctrl.Exports().(kubernetes.Exports)

			if len(actual.Data) != len(expect.Data) {
				return fmt.Errorf("expected %#v piece of data, got %#v", len(expect.Data), len(actual.Data))
			}

			if !reflect.DeepEqual(expect.Data, actual.Data) {
				return fmt.Errorf("expected %#v, got %#v", expect, actual)
			}
			return nil
		})
	}

	requireExports(t.Expected)
}

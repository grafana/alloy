package runtime_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/stretchr/testify/require"
)

func TestForeach(t *testing.T) {
	tt := []testCase{
		{
			name: "BasicForeach",
			config: `
			declare "sum_each" {
				argument "input_collection" {
					optional = false
					comment = "A list of numbers."
				}

				for_each = input_collection

				export "output" {
					value = testcomponents.summation.default.sum
				}

				testcomponents.summation "default"	{
					input = each.value
				}
			}

			sum_each "default" {
			}
			`,
			// expected: 10,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := runtime.New(testOptions(t))
			f, err := runtime.ParseSource(t.Name(), []byte(tc.config))
			require.NoError(t, err)
			require.NotNil(t, f)

			err = ctrl.LoadSource(f, nil)
			require.NoError(t, err)

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				ctrl.Run(ctx)
				close(done)
			}()
			defer func() {
				cancel()
				<-done
			}()

			args := []int{1, 2, 3, 4, 5}
			updateComponent(t, ctrl, "", "sum_each", args)

			require.Eventually(t, func() bool {
				export := getExport[[]int](t, ctrl, "", "foreach.default.output")
				// return export == tc.expected
				return reflect.DeepEqual(export, args)
			}, 3*time.Second, 10*time.Millisecond)

			args = []int{100, 100, 100, 100, 100}
			updateComponent(t, ctrl, "", "foreach.default", args)

			require.Eventually(t, func() bool {
				export := getExport[testcomponents.SummationExports](t, ctrl, "", "foreach.default.output")
				return reflect.DeepEqual(export, args)
			}, 3*time.Second, 10*time.Millisecond)
		})
	}
}

package runtime_test

import (
	"context"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name     string
	config   string
	expected int
}

func TestDeclare(t *testing.T) {
	tt := []testCase{
		{
			name: "BasicDeclare",
			config: `
			declare "test" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
			
				export "output" {
					value = testcomponents.passthrough.pt.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			test "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "NestedDeclares",
			config: `
			declare "test" {
				argument "input" {
					optional = false
				}
				declare "nested" {
					argument "input" {
						optional = false
					}
					export "output" {
						value = argument.input.value
					}
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
				nested "default" {
					input = testcomponents.passthrough.pt.output
				}
			
				export "output" {
					value = nested.default.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			test "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "DeclaredInParentDepth1",
			config: `
			declare "test" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
				rootDeclare "default" {
					input = testcomponents.passthrough.pt.output
				}
			
				export "output" {
					value = rootDeclare.default.output
				}
			}
			declare "rootDeclare" {
				argument "input" {
					optional = false
				}
				export "output" {
					value = argument.input.value
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			test "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "DeclaredInParentDepth2",
			config: `
			declare "test" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
				declare "anotherDeclare" {
					argument "input" {
						optional = false
					}
					rootDeclare "default" {
						input = argument.input.value
					}
					export "output" {
						value = rootDeclare.default.output
					}
				}
				anotherDeclare "myOtherDeclare" {
					input = testcomponents.passthrough.pt.output
				}
			
				export "output" {
					value = anotherDeclare.myOtherDeclare.output
				}
			}
			declare "rootDeclare" {
				argument "input" {
					optional = false
				}
				export "output" {
					value = argument.input.value
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			test "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "ShadowNamespace",
			config: `
			declare "prometheus" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
			
				export "output" {
					value = testcomponents.passthrough.pt.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			prometheus "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = prometheus.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "ShadowDeclare",
			config: `
			declare "a" {
				argument "input" {
					optional = false
				}
				export "output" {
					value = argument.input.value
				}
			}

			declare "test" {
				// redeclare "a"
				declare "a" {
					export "output" {
						value = -10
					}
				}
			
				a "default" {}
			
				export "output" {
					value = a.default.output
				}
			}
			test "myModule" {}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected: -10,
		},
		{
			name: "ShadowStdlib",
			config: `
			declare "constants" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
			
				export "output" {
					value = testcomponents.passthrough.pt.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			constants "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = constants.myModule.output
			}
			`,
			expected: 10,
		},
		{
			name: "CommunityComponent",
			config: `
			declare "com" {
				argument "input" {
					optional = false
				}

				testcomponents.community "default" {}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
			
				export "output" {
					value = testcomponents.passthrough.pt.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			com "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = com.myModule.output
			}
			`,
			expected: 10,
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

			require.Eventually(t, func() bool {
				export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
				return export.LastAdded == tc.expected
			}, 3*time.Second, 10*time.Millisecond)
		})
	}
}

type errorTestCase struct {
	name          string
	config        string
	expectedError *regexp.Regexp
}

func TestDeclareError(t *testing.T) {
	tt := []errorTestCase{
		{
			name: "CircleDependencyBetweenDeclares",
			config: `
			declare "a" {
				b "t1" {}
			}
			declare "b" {
				a "t2" {}
			}
			a "t3" {}
			`,
			// using regex here because the order of the node can vary
			// not ideal because it could technically match "a" and "a"
			expectedError: regexp.MustCompile(`cycle: declare\.(a|b), declare\.(a|b)`),
		},
		{
			name: "CircleDependencyWithinDeclare",
			config: `
			declare "a" {
				declare "b" {
					c "t1" {}
				}
				declare "c" {
					b "t2" {}
				}
				b "t3" {}
			}
			a "t4" {}
			`,
			expectedError: regexp.MustCompile(`cycle: declare\.(b|c), declare\.(b|c)`),
		},
		{
			name: "CircleDependencyWithItself",
			config: `
			declare "a" {
				a "t1" {}
			}
			a "t2" {}
			`,
			expectedError: regexp.MustCompile(`self reference: declare\.a`),
		},
		{
			name: "OutOfScopeReference",
			config: `
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}

			declare "example_a" {
				testcomponents.summation "sum" {
					input = testcomponents.count.inc.count // invalid reference
				}
			}
			example_a "test" {}
			`,
			expectedError: regexp.MustCompile(`component "testcomponents.count.inc.count" does not exist or is out of scope`),
		},
		{
			name: "OutOfScopeDefinition",
			config: `
			declare "a" {
				b_1 "default" { } // this should error 
			}
			declare "b" {
				declare "b_1" {}
			}
			a "example" {}
			`,
			expectedError: regexp.MustCompile(`cannot find the definition of component name "b_1"`),
		},
		{
			name: "ForbiddenDeclareLabel",
			config: `
			declare "declare" {}
			`,
			expectedError: regexp.MustCompile(`'declare' is not a valid label for a declare block`),
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer verifyNoGoroutineLeaks(t)
			s, err := logging.New(os.Stderr, logging.DefaultOptions)
			require.NoError(t, err)
			ctrl := runtime.New(runtime.Options{
				Logger:       s,
				DataPath:     t.TempDir(),
				MinStability: featuregate.StabilityPublicPreview,
				Reg:          nil,
				Services:     []service.Service{},
			})
			f, err := runtime.ParseSource(t.Name(), []byte(tc.config))
			require.NoError(t, err)
			require.NotNil(t, f)

			err = ctrl.LoadSource(f, nil)
			if err == nil {
				t.Errorf("Expected error to match regex %q, but got: nil", tc.expectedError)
			} else if !tc.expectedError.MatchString(err.Error()) {
				t.Errorf("Expected error to match regex %q, but got: %v", tc.expectedError, err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				ctrl.Run(ctx)
				close(done)
			}()
			cancel()
			<-done
		})
	}
}

type testCaseUpdateConfig struct {
	name        string
	config      string
	newConfig   string
	expected    int
	newExpected int
}

func TestDeclareUpdateConfig(t *testing.T) {
	tt := []testCaseUpdateConfig{
		{
			name: "UpdateDeclare",
			config: `
			declare "test" {
				argument "input" {
					optional = false
				}
			
				testcomponents.passthrough "pt" {
					input = argument.input.value
					lag = "1ms"
				}
			
				export "output" {
					value = testcomponents.passthrough.pt.output
				}
			}
			testcomponents.count "inc" {
				frequency = "10ms"
				max = 10
			}
		
			test "myModule" {
				input = testcomponents.count.inc.count
			}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			newConfig: `
			declare "test" {
				export "output" {
					value = -10
				}
			}
		
			test "myModule" {}
		
			testcomponents.summation "sum" {
				input = test.myModule.output
			}
			`,
			expected:    10,
			newExpected: -10,
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

			require.Eventually(t, func() bool {
				export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
				return export.LastAdded == tc.expected
			}, 3*time.Second, 10*time.Millisecond)

			f, err = runtime.ParseSource(t.Name(), []byte(tc.newConfig))
			require.NoError(t, err)
			require.NotNil(t, f)

			// Reload the controller with the new config.
			err = ctrl.LoadSource(f, nil)
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
				return export.LastAdded == tc.newExpected
			}, 3*time.Second, 10*time.Millisecond)
		})
	}
}

package features

import (
	"flag"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Example() {
	var (
		testFeature = Feature("test-feature")

		// Set of flags which require a specific feature to be enabled.
		dependencies = []Dependency{
			{Flag: "protected", Feature: testFeature},
		}
	)

	fs := flag.NewFlagSet("feature-flags", flag.PanicOnError)
	fs.String("protected", "", `Requires "test-feature" to be enabled to set.`)
	Register(fs, []Feature{testFeature})

	if err := fs.Parse([]string{"--protected", "foo"}); err != nil {
		fmt.Println(err)
	}

	err := Validate(fs, dependencies)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("Everything is valid!")
	}
	// Output: flag "protected" requires feature "test-feature" to be provided in --enable-features
}

var (
	exampleFeature  = Feature("test-feature")
	exampleFeatures = []Feature{exampleFeature}
)

func TestFeatures_Flag(t *testing.T) {
	fs := flag.NewFlagSet(t.Name(), flag.PanicOnError)
	Register(fs, exampleFeatures)

	f := fs.Lookup(setFlagName)
	require.Equal(t,
		"Comma-delimited list of features to enable. Valid values: test-feature",
		f.Usage,
	)

	t.Run("Exact match", func(t *testing.T) {
		err := f.Value.Set(string(exampleFeature))
		require.NoError(t, err)
		require.True(t, Enabled(fs, exampleFeature))
	})

	t.Run("Case insensitive", func(t *testing.T) {
		err := f.Value.Set(strings.ToUpper(string(exampleFeature)))
		require.NoError(t, err)
		require.True(t, Enabled(fs, exampleFeature))
	})

	t.Run("Feature does not exist", func(t *testing.T) {
		err := f.Value.Set(fmt.Sprintf("%s,bad-feature", exampleFeature))
		require.EqualError(t, err, `unknown feature "bad-feature". possible options: test-feature`)
	})
}

func TestValidate(t *testing.T) {
	tt := []struct {
		name    string
		input   []string
		enabled bool
		expect  error
	}{
		{
			name:    "Not enabled and not provided",
			input:   []string{},
			enabled: false,
			expect:  nil,
		},
		{
			name:    "Not enabled but provided",
			input:   []string{"--example-value", "foo"},
			enabled: false,
			expect:  fmt.Errorf(`flag "example-value" requires feature "test-feature" to be provided in --enable-features`),
		},
		{
			name: "Enabled and provided",
			input: []string{
				"--enable-features=test-feature",
				"--example-value", "foo",
			},
			enabled: true,
			expect:  nil,
		},
		{
			name: "Enabled and not provided",
			input: []string{
				"--enable-features=test-feature",
			},
			enabled: true,
			expect:  nil,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var exampleValue string

			fs := flag.NewFlagSet(t.Name(), flag.PanicOnError)
			fs.StringVar(&exampleValue, "example-value", "", "")
			Register(fs, exampleFeatures)

			err := fs.Parse(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.enabled, Enabled(fs, exampleFeature))

			err = Validate(fs, []Dependency{{
				Flag:    "example-value",
				Feature: exampleFeature,
			}})
			if tc.expect == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expect.Error())
			}
		})
	}
}

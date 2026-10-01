package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents"
)

func TestListComponentsIncludesFullyQualifiedComponentsFromImportedCustomComponentModules(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	ctrl, err := New(testOptions(t))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	source, err := ParseSource(t.Name(), []byte(`
		import.string "imported" {
			content = `+"`"+`declare "module" {
				testcomponents.tick "ticker" {
					frequency = "1s"
				}
			}`+"`"+`
		}

		imported.module "example" {}
	`))
	require.NoError(t, err)
	require.NoError(t, ctrl.LoadSource(source, nil, ""))
	require.Eventually(t, ctrl.LoadComplete, 2*time.Second, 100*time.Millisecond)

	opts := component.InfoOptions{GetHealth: true}
	var importedModuleID string
	require.Eventually(t, func() bool {
		components, err := ctrl.ListComponents("", opts)
		require.NoError(t, err)
		for _, info := range components {
			if info.ID.String() == "imported.module.example" && len(info.ModuleIDs) == 1 {
				importedModuleID = info.ModuleIDs[0]
				return true
			}
		}
		return false
	}, 2*time.Second, 100*time.Millisecond)

	require.Equal(t, "imported.module.example", importedModuleID)

	components, err := ctrl.ListComponents(importedModuleID, opts)
	require.NoError(t, err)
	require.Len(t, components, 1)
	require.Equal(t, importedModuleID, components[0].ID.ModuleID)
	require.Equal(t, "testcomponents.tick.ticker", components[0].ID.LocalID)
	require.Equal(t, "imported.module.example/testcomponents.tick.ticker", components[0].ID.String())
}

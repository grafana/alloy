package hierarchy

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// List will populate list with elements that match sel.
func List(ctx context.Context, cli client.Client, list client.ObjectList, sel Selector) error {
	if err := cli.List(ctx, list, sel); err != nil {
		return fmt.Errorf("list failed: %w", err)
	}
	if err := filterList(ctx, cli, list, sel); err != nil {
		return fmt.Errorf("filter failed: %w", err)
	}
	return nil
}

// filterList updates the provided list to only elements which match sel.
func filterList(ctx context.Context, cli client.Client, list client.ObjectList, sel Selector) error {
	allElements, err := meta.ExtractList(list)
	if err != nil {
		return fmt.Errorf("failed to get list: %w", err)
	}

	filtered := make([]runtime.Object, 0, len(allElements))
	for _, element := range allElements {
		obj, ok := element.(client.Object)
		if !ok {
			return fmt.Errorf("unexpected object of type %T in list", element)
		}

		matches, err := sel.Matches(ctx, cli, obj)
		if err != nil {
			return fmt.Errorf("failed to validate object: %w", err)
		}
		if matches {
			filtered = append(filtered, obj)
		}
	}

	if err := meta.SetList(list, filtered); err != nil {
		return fmt.Errorf("failed to update list: %w", err)
	}
	return nil
}

package hierarchy

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Selector finding objects within the resource hierarchy.
type Selector interface {
	// ListOption can be passed to List to perform initial filtering of returned
	// objects.
	client.ListOption

	// Matches returns true if the Selector matches the provided Object. The
	// provided Client may be used to perform extra searches.
	Matches(context.Context, client.Client, client.Object) (bool, error)
}

// LabelsSelector is used for discovering a set of objects in a hierarchy based
// on labels.
type LabelsSelector struct {
	// NamespaceName is the default namespace to search for objects in when
	// NamespaceSelector is nil.
	NamespaceName string

	// NamespaceLabels causes all namespaces whose labels match NamespaceLabels
	// to be searched. When nil, only the namespace specified by NamespaceName
	// will be searched.
	NamespaceLabels labels.Selector

	// Labels discovers all objects whose labels match the selector. If nil,
	// no objects will be discovered.
	Labels labels.Selector
}

var _ Selector = (*LabelsSelector)(nil)

// ApplyToList implements Selector.
func (ls *LabelsSelector) ApplyToList(lo *client.ListOptions) {
	if ls.NamespaceLabels == nil {
		lo.Namespace = ls.NamespaceName
	}
	lo.LabelSelector = ls.Labels
}

// Matches implements Selector.
func (ls *LabelsSelector) Matches(ctx context.Context, cli client.Client, o client.Object) (bool, error) {
	if !ls.Labels.Matches(labels.Set(o.GetLabels())) {
		return false, nil
	}

	// Fast path: we don't need to retrieve the labels of the namespace.
	if ls.NamespaceLabels == nil {
		return o.GetNamespace() == ls.NamespaceName, nil
	}

	// Slow path: we need to look up the namespace to see if its labels match. As
	// long as cli implements caching, this won't be too bad.
	var ns corev1.Namespace
	if err := cli.Get(ctx, client.ObjectKey{Name: o.GetNamespace()}, &ns); err != nil {
		return false, fmt.Errorf("error looking up namespace %q: %w", o.GetNamespace(), err)
	}
	return ls.NamespaceLabels.Matches(labels.Set(ns.GetLabels())), nil
}

// KeySelector is used for discovering a single object based on namespace and
// name.
type KeySelector struct {
	Namespace, Name string
}

var _ Selector = (*KeySelector)(nil)

// ApplyToList implements Selector.
func (ks *KeySelector) ApplyToList(lo *client.ListOptions) {
	lo.Namespace = ks.Namespace
}

// Matches implements Selector.
func (ks *KeySelector) Matches(ctx context.Context, cli client.Client, o client.Object) (bool, error) {
	return ks.Name == o.GetName() && ks.Namespace == o.GetNamespace(), nil
}

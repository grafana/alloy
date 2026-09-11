// Package all imports Alloy component packages for their registration side effects.
//
// By default, all known components are imported. Builds tagged with
// alloy_custom_components instead use generated, per-package imports selected
// from the builder manifest.
package all

//go:generate go run ./generate audit -repo-root ../../..
//go:generate go run ./generate generate -out .

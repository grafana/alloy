// Package all imports Alloy component packages for their registration side effects.
//
// By default, all known components are imported. Builds tagged with
// alloy_custom_components instead use the generated, per-package import files and
// can select components with alloy_component_* build tags.
package all

//go:generate go run ./generate audit -repo-root ../../..
//go:generate go run ./generate generate -out .

// Package component describes the interfaces which Flow components implement.
//
// A Flow component is a distinct piece of business logic that accepts inputs
// (Arguments) for its configuration and can optionally export a set of outputs
// (Exports).
//
// Arguments and Exports do not need to be static for the lifetime of a
// component. A component will be given a new Config if the runtime
// configuration changes. A component may also update its Exports throughout
// its lifetime, such as a component which outputs the current day of the week.
//
// Components are built by users with River, where they can use River
// expressions to refer to any input or exported field from other components.
// This allows users to connect components together to declaratively form a
// pipeline.
//
// # Defining Arguments and Exports structs
//
// Arguments and Exports implemented by new components must be able to be
// encoded to and from River. "river" struct field tags are used for encoding;
// refer to the package documentation at pkg/river for a description of how to
// write these tags.
//
// The set of River element names of a given component's Arguments and Exports
// types must not overlap. Additionally, the following River field and block
// names are reserved for use by the Flow controller:
//
//   - for_each
//   - enabled
//   - health
//   - debug
//
// Default values for Arguments may be provided by implementing
// river.Unmarshaler.
//
// # Arguments and Exports immutability
//
// Arguments passed to a component should be treated as immutable, as memory
// can be shared between components as an optimization. Components should make
// copies for fields they need to modify. An exception to this is for fields
// which are expected to be mutable (e.g., interfaces which expose a
// goroutine-safe API).
//
// Similarly, Exports and the fields within Exports must be considered
// immutable after they are written for the same reason.
//
// # Mapping River strings to custom types
//
// Custom encoding and decoding of fields is available by implementing
// encoding.TextMarshaler and encoding.TextUnmarshaler. Types implementing
// these interfaces will be represented as strings in River.
//
// # Component registration
//
// Components are registered globally by calling Register. These components are
// then made available by including them in the import path. The "all" child
// package imports all known component packages and should be updated when
// creating a new one.
package component

import (
	"context"
)

// The Arguments contains the input fields for a specific component, which is
// unmarshaled from River.
//
// Refer to the package documentation for details around how to build proper
// Arguments implementations.
type Arguments interface{}

// Exports contains the current set of outputs for a specific component, which
// is then marshaled to River.
//
// Refer to the package documentation for details around how to build proper
// Exports implementations.
type Exports interface{}

// Component is the base interface for a Flow component. Components may
// implement extension interfaces (named <Extension>Component) to implement
// extra known behavior.
type Component interface {
	// Run starts the component, blocking until ctx is canceled or the component
	// suffers a fatal error. Run is guaranteed to be called exactly once per
	// Component.
	//
	// Implementations of Component should perform any necessary cleanup before
	// returning from Run.
	Run(ctx context.Context) error

	// Update provides a new Config to the component. The type of newConfig will
	// always match the struct type which the component registers.
	//
	// Update will be called concurrently with Run. The component must be able to
	// gracefully handle updating its config while still running.
	//
	// An error may be returned if the provided config is invalid.
	Update(args Arguments) error
}

// DebugComponent is an extension interface for components which can report
// debugging information upon request.
type DebugComponent interface {
	Component

	// DebugInfo returns the current debug information of the component. May
	// return nil if there is no debug info to currently report. The result of
	// DebugInfo must be encodable to River like Arguments and Exports.
	//
	// Values from DebugInfo are not exposed to other components for use in
	// expressions.
	//
	// DebugInfo must be safe for calling concurrently.
	DebugInfo() interface{}
}

package component

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/grafana/river/encoding/riverjson"
)

var (
	// ErrComponentNotFound is returned by [Provider.GetComponent] when the
	// specified component isn't found.
	ErrComponentNotFound = errors.New("component not found")

	// ErrModuleNotFound is returned by [Provider.ListComponents] when the
	// specified module isn't found.
	ErrModuleNotFound = errors.New("module not found")
)

// A Provider is a system which exposes a list of running components.
type Provider interface {
	// GetComponent returns information about an individual running component
	// given its global ID. The provided opts field configures how much detail to
	// return; see [InfoOptions] for more information.
	//
	// GetComponent returns ErrComponentNotFound if a component is not found.
	GetComponent(id ID, opts InfoOptions) (*Info, error)

	// ListComponents returns the list of active components. The provided opts
	// field configures how much detail to return; see [InfoOptions] for more
	// information.
	//
	// Returns ErrModuleNotFound if the provided moduleID doesn't exist.
	ListComponents(moduleID string, opts InfoOptions) ([]*Info, error)
}

// ID is a globally unique identifier for a component.
type ID struct {
	ModuleID string // Unique ID of the module that the component is running in.
	LocalID  string // Local ID of the component, unique to the module it is running in.
}

// ParseID parses an input string of the form "LOCAL_ID" or
// "MODULE_ID/LOCAL_ID" into an ID. The final slash character is used to
// separate the ModuleID and LocalID.
func ParseID(input string) ID {
	slashIndex := strings.LastIndexByte(input, '/')
	if slashIndex == -1 {
		return ID{LocalID: input}
	}
	return ID{
		ModuleID: input[:slashIndex],
		LocalID:  input[slashIndex+1:],
	}
}

// InfoOptions is used by to determine how much information to return with
// [Info].
type InfoOptions struct {
	GetHealth    bool // When true, sets the Health field of returned components.
	GetArguments bool // When true, sets the Arguments field of returned components.
	GetExports   bool // When true, sets the Exports field of returned components.
	GetDebugInfo bool // When true, sets the DebugInfo field of returned components.
}

// String returns the "<ModuleID>/<LocalID>" string representation of the id.
func (id ID) String() string {
	if id.ModuleID == "" {
		return id.LocalID
	}
	return id.ModuleID + "/" + id.LocalID
}

// Info ia detailed information about a component.
type Info struct {
	// Component is the instance of the component. Component may be nil if a
	// component exists in the controller's DAG but it has not been successfully
	// evaluated yet.
	Component Component

	// ModuleIDs includes the list of current module IDs that the component is
	// running. Module IDs are always globally unique.
	//
	// The sort order of the list is not guaranteed.
	ModuleIDs []string

	ID    ID     // ID of the component.
	Label string // Component label. Not set for singleton components.

	// References and ReferencedBy are the list of IDs in the same module that
	// this component depends on, or is depended on by, respectively.
	References, ReferencedBy []string

	ComponentName string // Name of the component.
	Health        Health // Current component health.

	Arguments Arguments   // Current arguments value of the component.
	Exports   Exports     // Current exports value of the component.
	DebugInfo interface{} // Current debug info of the component.
}

// MarshalJSON returns a JSON representation of cd. The format of the
// representation is not stable and is subject to change.
func (info *Info) MarshalJSON() ([]byte, error) {
	type (
		componentHealthJSON struct {
			State       string    `json:"state"`
			Message     string    `json:"message"`
			UpdatedTime time.Time `json:"updatedTime"`
		}

		componentDetailJSON struct {
			Name             string               `json:"name"`
			Type             string               `json:"type,omitempty"`
			LocalID          string               `json:"localID"`
			ModuleID         string               `json:"moduleID"`
			Label            string               `json:"label,omitempty"`
			References       []string             `json:"referencesTo"`
			ReferencedBy     []string             `json:"referencedBy"`
			Health           *componentHealthJSON `json:"health"`
			Original         string               `json:"original"`
			Arguments        json.RawMessage      `json:"arguments,omitempty"`
			Exports          json.RawMessage      `json:"exports,omitempty"`
			DebugInfo        json.RawMessage      `json:"debugInfo,omitempty"`
			CreatedModuleIDs []string             `json:"createdModuleIDs,omitempty"`
		}
	)

	var (
		references   = info.References
		referencedBy = info.ReferencedBy

		arguments, exports, debugInfo json.RawMessage
		err                           error
	)

	if references == nil {
		references = []string{}
	}
	if referencedBy == nil {
		referencedBy = []string{}
	}

	arguments, err = riverjson.MarshalBody(info.Arguments)
	if err != nil {
		return nil, err
	}
	exports, err = riverjson.MarshalBody(info.Exports)
	if err != nil {
		return nil, err
	}
	debugInfo, err = riverjson.MarshalBody(info.DebugInfo)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&componentDetailJSON{
		Name:         info.ComponentName,
		Type:         "block",
		ModuleID:     info.ID.ModuleID,
		LocalID:      info.ID.LocalID,
		Label:        info.Label,
		References:   references,
		ReferencedBy: referencedBy,
		Health: &componentHealthJSON{
			State:       info.Health.Health.String(),
			Message:     info.Health.Message,
			UpdatedTime: info.Health.UpdateTime,
		},
		Arguments:        arguments,
		Exports:          exports,
		DebugInfo:        debugInfo,
		CreatedModuleIDs: info.ModuleIDs,
	})
}

// GetAllComponents enumerates over all of the modules in p and returns the set
// of all components.
func GetAllComponents(p Provider, opts InfoOptions) []*Info {
	return getAllComponentsByModule("", p, opts)
}

func getAllComponentsByModule(moduleID string, p Provider, opts InfoOptions) []*Info {
	var components []*Info

	// ListComponents may return an error here if the module went away since the
	// time we were given the ID, so we'll ignore it.
	infos, err := p.ListComponents(moduleID, opts)
	if err != nil {
		return components
	}

	for _, info := range infos {
		components = append(components, info)

		for _, module := range info.ModuleIDs {
			moduleComponents := getAllComponentsByModule(module, p, opts)
			components = append(components, moduleComponents...)
		}
	}

	return components
}

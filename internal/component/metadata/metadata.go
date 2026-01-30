package metadata

import (
	"fmt"
	"reflect"

	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/all"
	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/pyroscope"
)

//TODO(thampiotr): Instead of metadata package reaching into registry, we'll migrate to using a YAML schema file that
//				   contains information about all the available components. This file will be generated separately and
//				   can be used by other tools.

type Type struct {
	Name string
	// Returns true if provided args include this type (including nested structs)
	existsInArgsFn func(args component.Arguments) bool
	// Returns true if provided exports include this type (including nested structs)
	existsInExportsFn func(exports component.Exports) bool
}

func (t Type) String() string {
	return fmt.Sprintf("Type[%s]", t.Name)
}

var (
	TypeTargets = Type{
		Name: "Targets",
		existsInArgsFn: func(args component.Arguments) bool {
			return hasFieldOfType(args, reflect.TypeOf([]discovery.Target{}))
		},
		existsInExportsFn: func(exports component.Exports) bool {
			return hasFieldOfType(exports, reflect.TypeOf([]discovery.Target{}))
		},
	}

	TypeLokiLogs = Type{
		Name: "Loki `LogsReceiver`",
		existsInArgsFn: func(args component.Arguments) bool {
			return hasFieldOfType(args, reflect.TypeOf([]loki.LogsReceiver{}))
		},
		existsInExportsFn: func(exports component.Exports) bool {
			return hasFieldOfType(exports, reflect.TypeOf(loki.NewLogsReceiver()))
		},
	}

	TypePromMetricsReceiver = Type{
		Name: "Prometheus `MetricsReceiver`",
		existsInArgsFn: func(args component.Arguments) bool {
			return hasFieldOfType(args, reflect.TypeOf([]storage.Appendable{}))
		},
		existsInExportsFn: func(exports component.Exports) bool {
			var a *storage.Appendable = nil
			return hasFieldOfType(exports, reflect.TypeOf(a).Elem())
		},
	}

	TypePyroProfilesReceiver = Type{
		Name: "Pyroscope `ProfilesReceiver`",
		existsInArgsFn: func(args component.Arguments) bool {
			return hasFieldOfType(args, reflect.TypeOf([]pyroscope.Appendable{}))
		},
		existsInExportsFn: func(exports component.Exports) bool {
			var a *pyroscope.Appendable = nil
			return hasFieldOfType(exports, reflect.TypeOf(a).Elem())
		},
	}

	TypeOTELReceiver = Type{
		Name: "OpenTelemetry `otelcol.Consumer`",
		existsInArgsFn: func(args component.Arguments) bool {
			return hasFieldOfType(args, reflect.TypeOf([]otelcol.Consumer{}))
		},
		existsInExportsFn: func(exports component.Exports) bool {
			var a *otelcol.Consumer = nil
			return hasFieldOfType(exports, reflect.TypeOf(a).Elem())
		},
	}

	AllTypes = []Type{
		TypeTargets,
		TypeLokiLogs,
		TypePromMetricsReceiver,
		TypePyroProfilesReceiver,
		TypeOTELReceiver,
	}
)

type Metadata struct {
	accepts []Type
	exports []Type
}

func (m Metadata) Empty() bool {
	return len(m.accepts) == 0 && len(m.exports) == 0
}

func (m Metadata) AllTypesAccepted() []Type {
	return m.accepts
}

func (m Metadata) AllTypesExported() []Type {
	return m.exports
}

func (m Metadata) AcceptsType(t Type) bool {
	for _, a := range m.accepts {
		if a.Name == t.Name {
			return true
		}
	}
	return false
}

func (m Metadata) ExportsType(t Type) bool {
	for _, o := range m.exports {
		if o.Name == t.Name {
			return true
		}
	}
	return false
}

func ForComponent(name string) (Metadata, error) {
	reg, ok := component.Get(name)
	if !ok {
		return Metadata{}, fmt.Errorf("could not find component %q", name)
	}
	return inferMetadata(reg.Args, reg.Exports), nil
}

func inferMetadata(args component.Arguments, exports component.Exports) Metadata {
	m := Metadata{}
	for _, t := range AllTypes {
		if t.existsInArgsFn(args) {
			m.accepts = append(m.accepts, t)
		}
		if t.existsInExportsFn(exports) {
			m.exports = append(m.exports, t)
		}
	}
	return m
}

func hasFieldOfType(obj any, fieldType reflect.Type) bool {
	objValue := reflect.ValueOf(obj)

	// If the object is a pointer, dereference it
	for objValue.Kind() == reflect.Pointer {
		objValue = objValue.Elem()
	}

	// If the object is not a struct or interface, return false
	if objValue.Kind() != reflect.Struct && objValue.Kind() != reflect.Interface {
		return false
	}

	for i := 0; i < objValue.NumField(); i++ {
		fv := objValue.Field(i)
		ft := fv.Type()

		// If the field type matches the given type, return true
		if ft == fieldType {
			return true
		}

		if fv.Kind() == reflect.Interface && fieldType.AssignableTo(ft) {
			return true
		}

		// If the field is a struct, recursively check its fields
		if fv.Kind() == reflect.Struct {
			if fv.CanInterface() {
				if hasFieldOfType(fv.Interface(), fieldType) {
					return true
				}
			} else {
				return false
			}
		}

		// If the field is a pointer, create a new instance of the pointer type and recursively check its fields
		if fv.Kind() == reflect.Pointer {
			if hasFieldOfType(reflect.New(ft.Elem()).Interface(), fieldType) {
				return true
			}
		}
	}

	return false
}

package integrations

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/grafana/alloy/internal/static/integrations/config"
	"github.com/grafana/alloy/internal/util"
	"gopkg.in/yaml.v2"
)

var (
	registeredIntegrations = []Config{}
	configFieldNames       = make(map[reflect.Type]string)

	emptyStructType = reflect.TypeOf(struct{}{})
	configsType     = reflect.TypeOf(Configs{})
)

// RegisterIntegration dynamically registers a new integration. The Config
// will represent the configuration that controls the specific integration.
// Registered Configs may be loaded using UnmarshalYAML or manually
// constructed.
//
// RegisterIntegration panics if cfg is not a pointer.
func RegisterIntegration(cfg Config) {
	if reflect.TypeOf(cfg).Kind() != reflect.Pointer {
		panic(fmt.Sprintf("RegisterIntegration must be given a pointer, got %T", cfg))
	}
	registeredIntegrations = append(registeredIntegrations, cfg)
	configFieldNames[reflect.TypeOf(cfg)] = cfg.Name()
}

// RegisteredIntegrations all Configs that were passed to RegisterIntegration.
// Each call will generate a new set of pointers.
func RegisteredIntegrations() []Config {
	res := make([]Config, 0, len(registeredIntegrations))
	for _, in := range registeredIntegrations {
		res = append(res, cloneIntegration(in))
	}
	return res
}

func cloneIntegration(c Config) Config {
	return reflect.New(reflect.TypeOf(c).Elem()).Interface().(Config)
}

// Configs is a list of UnmarshaledConfig. Configs for integrations which are
// unmarshaled from YAML are combined with common settings.
type Configs []UnmarshaledConfig

// UnmarshaledConfig combines an integration's config with common settings.
type UnmarshaledConfig struct {
	Config
	Common config.Common
}

// MarshalYAML helps implement yaml.Marshaller for structs that have a Configs
// field that should be inlined in the YAML string.
func MarshalYAML(v any) (any, error) {
	inVal := reflect.ValueOf(v)
	for inVal.Kind() == reflect.Pointer {
		inVal = inVal.Elem()
	}
	if inVal.Kind() != reflect.Struct {
		return nil, fmt.Errorf("integrations: can only marshal a struct, got %T", v)
	}
	inType := inVal.Type()

	var (
		cfgType    = getConfigTypeForIntegrations(registeredIntegrations, inType)
		cfgPointer = reflect.New(cfgType)
		cfgVal     = cfgPointer.Elem()
	)

	// Copy over any existing value from inVal to cfgVal.
	//
	// The ordering of fields in inVal and cfgVal match identically up until the
	// extra fields appended to the end of cfgVal.
	var configs Configs
	for i, n := 0, inType.NumField(); i < n; i++ {
		if inType.Field(i).Type == configsType {
			configs = inVal.Field(i).Interface().(Configs)
			if configs == nil {
				configs = Configs{}
			}
		}
		if cfgType.Field(i).PkgPath != "" {
			continue // Field is unexported: ignore.
		}
		cfgVal.Field(i).Set(inVal.Field(i))
	}
	if configs == nil {
		return nil, fmt.Errorf("integrations: Configs field not found in type: %T", v)
	}

	for _, c := range configs {
		fieldName, ok := configFieldNames[reflect.TypeOf(c.Config)]
		if !ok {
			return nil, fmt.Errorf("integrations: cannot marshal unregistered Config type: %T", c)
		}
		field := cfgVal.FieldByName("XXX_Config_" + fieldName)
		rawConfig, err := getRawIntegrationConfig(c)
		if err != nil {
			return nil, fmt.Errorf("integrations: cannot marshal integration %q: %w", c.Name(), err)
		}
		field.Set(rawConfig)
	}

	return cfgPointer.Interface(), nil
}

// getRawIntegrationConfig turns an UnmarshaledConfig into the *util.RawYAML
// used to represent it in configs.
func getRawIntegrationConfig(uc UnmarshaledConfig) (v reflect.Value, err error) {
	bb, err := util.MarshalYAMLMerged(uc.Common, uc.Config)
	if err != nil {
		return v, err
	}
	raw := util.RawYAML(bb)
	return reflect.ValueOf(&raw), nil
}

// UnmarshalYAML helps implement yaml.Unmarshaller for structs that have a
// Configs field that should be inlined in the YAML string.
func UnmarshalYAML(out any, unmarshal func(any) error) error {
	return unmarshalIntegrationsWithList(registeredIntegrations, out, unmarshal)
}

// unmarshalIntegrationsWithList unmarshals to a subtype of out that has a
// field added for every integration in integrations. Code adapted from
// Prometheus:
//
//	https://github.com/prometheus/prometheus/blob/511511324adfc4f4178f064cc104c2deac3335de/discovery/registry.go#L111
func unmarshalIntegrationsWithList(integrations []Config, out any, unmarshal func(any) error) error {
	outVal := reflect.ValueOf(out)
	if outVal.Kind() != reflect.Pointer {
		return fmt.Errorf("integrations: can only unmarshal into a struct pointer, got %T", out)
	}
	outVal = outVal.Elem()
	if outVal.Kind() != reflect.Struct {
		return fmt.Errorf("integrations: can only unmarshal into a struct pointer, got %T", out)
	}
	outType := outVal.Type()

	var (
		cfgType    = getConfigTypeForIntegrations(integrations, outType)
		cfgPointer = reflect.New(cfgType)
		cfgVal     = cfgPointer.Elem()
	)

	// Copy over any existing value from outVal to cfgVal.
	//
	// The ordering of fields in outVal and cfgVal match identically up until the
	// extra fields appended to the end of cfgVal.
	var configs *Configs
	for i := 0; i < outVal.NumField(); i++ {
		if outType.Field(i).Type == configsType {
			if configs != nil {
				return fmt.Errorf("integrations: Multiple Configs fields found in %T", out)
			}
			configs = outVal.Field(i).Addr().Interface().(*Configs)
			continue
		}
		if cfgType.Field(i).PkgPath != "" {
			// Ignore unexported fields
			continue
		}
		cfgVal.Field(i).Set(outVal.Field(i))
	}
	if configs == nil {
		return fmt.Errorf("integrations: No Configs field found in %T", out)
	}

	// Unmarshal into our dynamic type.
	if err := unmarshal(cfgPointer.Interface()); err != nil {
		return replaceYAMLTypeError(err, cfgType, outType)
	}

	// Copy back unmarshaled fields that were originally in outVal.
	for i := 0; i < outVal.NumField(); i++ {
		if cfgType.Field(i).PkgPath != "" {
			// Ignore unexported fields
			continue
		}
		outVal.Field(i).Set(cfgVal.Field(i))
	}

	// Iterate through the remainder of our fields, which should all be dynamic
	// structs where the first field is a config.Common and the second field is
	// a Config.
	integrationLookup := buildIntegrationsMap(integrations)
	for i := outVal.NumField(); i < cfgVal.NumField(); i++ {
		// Our integrations are unmarshaled as *util.RawYAML. If it's nil, we treat
		// it as not defined.
		fieldType := cfgVal.Type().Field(i)
		field := cfgVal.Field(i)
		if field.IsNil() {
			continue
		}

		configName := strings.TrimPrefix(fieldType.Name, "XXX_Config_")
		configReference, ok := integrationLookup[configName]
		if !ok {
			return fmt.Errorf("integration %q not registered", configName)
		}
		uc, err := buildUnmarshaledConfig(field.Interface().(*util.RawYAML), configReference)
		if err != nil {
			return fmt.Errorf("failed to unmarshal integration %q: %w", configName, err)
		}
		*configs = append(*configs, uc)
	}

	return nil
}

// getConfigTypeForIntegrations returns a dynamic struct type that has all of
// the same fields as out including the fields for the provided integrations.
//
// integrations are unmarshaled to *util.RawYAML for deferred unmarshaling.
func getConfigTypeForIntegrations(integrations []Config, out reflect.Type) reflect.Type {
	// Initial exported fields map one-to-one.
	var fields []reflect.StructField
	for i, n := 0, out.NumField(); i < n; i++ {
		switch field := out.Field(i); {
		case field.PkgPath == "" && field.Type != configsType:
			fields = append(fields, field)
		default:
			fields = append(fields, reflect.StructField{
				Name:    "_" + field.Name, // Field must be unexported.
				PkgPath: out.PkgPath(),
				Type:    emptyStructType,
			})
		}
	}
	for _, cfg := range integrations {
		// Use a prefix that's unlikely to collide with anything else.
		fieldName := "XXX_Config_" + cfg.Name()
		fields = append(fields, reflect.StructField{
			Name: fieldName,
			Tag:  reflect.StructTag(fmt.Sprintf(`yaml:"%s,omitempty"`, cfg.Name())),
			Type: reflect.PointerTo(reflect.TypeOf(util.RawYAML{})),
		})
	}
	return reflect.StructOf(fields)
}

func buildIntegrationsMap(in []Config) map[string]Config {
	m := make(map[string]Config, len(in))
	for _, i := range in {
		m[i.Name()] = i
	}
	return m
}

// buildUnmarshaledConfig converts raw YAML into an UnmarshaledConfig where the
// config type is the same as ref.
func buildUnmarshaledConfig(raw *util.RawYAML, ref Config) (uc UnmarshaledConfig, err error) {
	// Initialize uc.Config so it can be unmarshaled properly as an interface.
	uc = UnmarshaledConfig{
		Config: cloneIntegration(ref),
	}
	err = util.UnmarshalYAMLMerged(*raw, &uc.Common, uc.Config)
	return
}

func replaceYAMLTypeError(err error, oldTyp, newTyp reflect.Type) error {
	if e, ok := err.(*yaml.TypeError); ok {
		oldStr := oldTyp.String()
		newStr := newTyp.String()
		for i, s := range e.Errors {
			e.Errors[i] = strings.ReplaceAll(s, oldStr, newStr)
		}
	}
	return err
}

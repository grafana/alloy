package config

// Exceptions configures runtime exception capture for the selected services.
type Exceptions struct {
	Enabled *bool         `alloy:"enabled,attr,optional"`
	Go      *GoExceptions `alloy:"go,block,optional"`
}

type GoExceptions struct {
	Panics          *bool `alloy:"panics,attr,optional"`
	FormattedErrors *bool `alloy:"formatted_errors,attr,optional"`
}

func (e *Exceptions) Convert() map[string]any {
	if e == nil {
		return nil
	}
	m := make(map[string]any)
	if e.Enabled != nil {
		m["enabled"] = *e.Enabled
	}
	if e.Go != nil {
		g := make(map[string]any)
		if e.Go.Panics != nil {
			g["panics"] = *e.Go.Panics
		}
		if e.Go.FormattedErrors != nil {
			g["formatted_errors"] = *e.Go.FormattedErrors
		}
		set(m, "go", g)
	}
	return m
}

func (e *Exceptions) enabled(inherited bool) bool {
	if e != nil && e.Enabled != nil {
		return *e.Enabled
	}
	return inherited
}

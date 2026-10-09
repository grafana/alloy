// Command componentpkgs prints a JSON object which maps the name of every
// native Alloy component to the Go package which registers it.
//
// It is used by the generator to resolve the component names listed in the
// "alloy" section of builder-config.yaml. It lives in its own program so that
// the generator only needs to compile every component when a component list is
// actually given.
package main

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/all"
)

func main() {
	pkgs := make(map[string]string)
	for _, name := range component.AllNames() {
		reg, _ := component.Get(name)
		pkg, err := packageOf(reg.Build)
		if err != nil {
			log.Fatalf("component %q: %v", name, err)
		}
		pkgs[name] = pkg
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(pkgs); err != nil {
		log.Fatal(err)
	}
}

// packageOf returns the import path of the package which defines fn.
func packageOf(fn any) (string, error) {
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return "", os.ErrNotExist
	}

	// Function names look like "github.com/grafana/alloy/internal/component/loki/source/file.init.0.func1".
	// The package path ends at the first dot after the last slash.
	name := f.Name()
	slash := strings.LastIndex(name, "/")
	dot := strings.Index(name[slash+1:], ".")
	if dot < 0 {
		return name, nil
	}
	return name[:slash+1+dot], nil
}

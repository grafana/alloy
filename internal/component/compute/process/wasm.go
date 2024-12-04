package process

import (
	"context"
	"fmt"
	"github.com/tetratelabs/wazero"
	"sync"
	"time"

	extism "github.com/extism/go-sdk"
)

type WasmPlugin struct {
	mut    sync.Mutex
	plugin *extism.Plugin
	config map[string]string
}

func NewPlugin(wasm []byte, wasmConfig map[string]string, ctx context.Context) (*WasmPlugin, error) {
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{
				Data: wasm,
			},
		},
	}

	mv := wazero.NewModuleConfig().WithSysWalltime()
	config := extism.PluginConfig{
		EnableWasi:   true,
		ModuleConfig: mv,
	}
	plugin, err := extism.NewPlugin(ctx, manifest, config, []extism.HostFunction{})
	if err != nil {
		return nil, err
	}
	return &WasmPlugin{
		plugin: plugin,
		config: wasmConfig,
	}, nil
}

func (wp *WasmPlugin) UpdateConfig(config map[string]string) {
	wp.mut.Lock()
	defer wp.mut.Unlock()

	wp.config = config
}

func (wp *WasmPlugin) Process(pt *Passthrough) (*Passthrough, error) {
	// Only one process can be happening at a time.
	wp.mut.Lock()
	defer wp.mut.Unlock()

	pt.Config = wp.config
	bb, err := pt.MarshalVT()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	ctx, cncl := context.WithTimeout(ctx, 2*time.Second)
	defer cncl()
	code, out, err := wp.plugin.CallWithContext(ctx, "process", bb)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("plugin returned non-zero code: %d", code)
	}
	returnPT := &Passthrough{}
	err = returnPT.UnmarshalVT(out)
	return returnPT, err
}

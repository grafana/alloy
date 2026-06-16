package alloyengine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/flowcmd"
	"github.com/grafana/alloy/internal/readyctx"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

var _ extension.Extension = (*alloyEngineExtension)(nil)

// running tracks whether any alloyengine instance is currently active.
var running = atomic.NewBool(false)

type state int

const (
	stateNotStarted state = iota
	stateStarting
	stateRunning
	stateShuttingDown
	stateTerminated
	stateRunError
)

func (e *alloyEngineExtension) setState(newState state) {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	oldState := e.state
	e.state = newState
	if oldState != newState {
		e.settings.Logger.Info("alloyengine extension state changed", zap.String("from", oldState.String()), zap.String("to", newState.String()))
	}
}

func (e *alloyEngineExtension) getState() state {
	e.stateMutex.Lock()
	defer e.stateMutex.Unlock()
	return e.state
}

func (s state) String() string {
	switch s {
	case stateNotStarted:
		return "not_started"
	case stateStarting:
		return "starting"
	case stateRunning:
		return "running"
	case stateShuttingDown:
		return "shutting_down"
	case stateTerminated:
		return "terminated"
	case stateRunError:
		return "run_error"
	}
	return fmt.Sprintf("unknown_state_%d", s)
}

// alloyEngineExtension implements the alloyengine extension.
type alloyEngineExtension struct {
	config            *Config
	settings          component.TelemetrySettings
	runExited         chan struct{}
	runCommandFactory func(modulePath string, configs map[string][]byte) *cobra.Command

	stateMutex sync.Mutex
	state      state
	runCancel  context.CancelFunc
}

// newAlloyEngineExtension creates a new alloyEngine extension instance.
func newAlloyEngineExtension(config *Config, settings component.TelemetrySettings) *alloyEngineExtension {
	return &alloyEngineExtension{
		config:            config,
		settings:          settings,
		state:             stateNotStarted,
		runCommandFactory: flowcmd.RunAsExtensionCommand,
	}
}

func (e *alloyEngineExtension) Start(_ context.Context, host component.Host) error {
	currentState := e.getState()
	switch currentState {
	case stateNotStarted:
		break
	default:
		return fmt.Errorf("cannot start alloyengine extension in current state: %s", currentState)
	}

	modulePath, files, err := buildAlloyConfig(e.config.AlloyConfig)
	if err != nil {
		return err
	}

	runCommand := e.runCommandFactory(modulePath, files)

	// Prevent cobra from autoloading command-line args from os.Args
	runCommand.SetArgs([]string{})

	if err := runCommand.ParseFlags(e.config.flagsAsSlice()); err != nil {
		return fmt.Errorf("failed to parse flags: %w", err)
	}

	// Here we check if another extension instance is already running, if so we return an error
	if !running.CompareAndSwap(false, true) {
		return fmt.Errorf("only one alloyengine extension can be active per process; an instance is already running")
	}

	runCtx, runCancel := context.WithCancel(context.Background())
	e.runCancel = runCancel
	e.runExited = make(chan struct{})

	runCtx = readyctx.WithOnReady(runCtx, func() {
		e.setState(stateRunning)
	})

	e.setState(stateStarting)

	go func() {
		defer func() {
			running.Store(false)
			close(e.runExited)
		}()

		err := e.runWithBackoffRetry(runCommand, runCtx)

		previousState := e.getState()
		e.setState(stateTerminated)

		if err == nil {
			e.settings.Logger.Debug("run command exited successfully without error")
		} else if previousState == stateShuttingDown {
			e.settings.Logger.Warn("run command exited with an error during shutdown", zap.Error(err))
		}
	}()
	return nil
}

func (e *alloyEngineExtension) runWithBackoffRetry(runCommand *cobra.Command, ctx context.Context) error {
	var lastError error
	baseDelay := 1 * time.Second
	i := 1

	for {
		err := runCommand.ExecuteContext(ctx)

		if err == nil {
			return nil
		}

		lastError = err
		e.setState(stateRunError)

		// exponential backoff until 15s
		delay := 15 * time.Second
		if i < 4 {
			delay = time.Duration(math.Pow(2, float64(i))) * baseDelay
		}

		e.settings.Logger.Warn("run command failed, will retry", zap.Error(err), zap.Duration("retry_delay", delay), zap.Int("attempt", i))

		i++

		select {
		case <-time.After(delay):
			// continue to next iteration
		case <-ctx.Done():
			return lastError
		}
	}
}

// Shutdown is called when the extension is being stopped.
func (e *alloyEngineExtension) Shutdown(ctx context.Context) error {
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		e.settings.Logger.Info("alloyengine extension shutting down")
		e.setState(stateShuttingDown)
		// guaranteed to be non-nil since runCancel is set in Start()
		e.runCancel()

		select {
		case <-e.runExited:
			e.settings.Logger.Info("alloyengine extension shut down successfully")
		case <-ctx.Done():
			e.settings.Logger.Warn("alloyengine extension shutdown interrupted by context", zap.Error(ctx.Err()))
		}
		return nil
	case stateNotStarted:
		e.settings.Logger.Info("alloyengine extension shutdown completed (not started)")
		return nil
	default:
		e.settings.Logger.Warn("alloyengine extension shutdown in current state is a no-op", zap.String("state", e.getState().String()))
		return nil
	}
}

// Ready returns nil when the extension is ready to process data.
func (e *alloyEngineExtension) Ready() error {
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", currentState.String())
	}
}

// NotReady returns an error when the extension is not ready to process data.
func (e *alloyEngineExtension) NotReady() error {
	currentState := e.getState()
	switch currentState {
	case stateStarting, stateRunning, stateRunError:
		return nil
	default:
		return fmt.Errorf("alloyengine extension not ready in current state: %s", currentState.String())
	}
}

func buildAlloyConfig(cfg AlloyConfig) (modulePath string, files map[string][]byte, err error) {
	isInlineConfig := cfg.Inline.Content != ""
	if isInlineConfig {
		if cfg.Path != "" {
			return "", nil, errors.New("exactly one of config.file or config.inline.content must be set")
		}

		modulePath = cfg.Inline.ModulePath
		if modulePath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", nil, fmt.Errorf("cannot get current working directory: %w", err)
			}
			modulePath = cwd
		}

		data := []byte(cfg.Inline.Content)
		if err := validateAlloyConfig("config.alloy", data); err != nil {
			return "", nil, fmt.Errorf("invalid inline Alloy config: %w", err)
		}

		files = map[string][]byte{"config.alloy": data}
		return modulePath, files, nil
	}

	// Alloy supports accepting a directory as config source
	stat, err := os.Lstat(cfg.Path)
	if err != nil {
		return "", nil, err
	}
	if !stat.IsDir() {
		modulePath = filepath.Dir(cfg.Path)
		data, err := os.ReadFile(cfg.Path)
		if err != nil {
			return "", nil, fmt.Errorf("failed to read alloy config file %q: %w", cfg.Path, err)
		}

		if err := validateAlloyConfig(cfg.Path, data); err != nil {
			return "", nil, fmt.Errorf("error in Alloy config file %q: %w", cfg.Path, err)
		}

		files = map[string][]byte{cfg.Path: data}
		return modulePath, files, nil
	}

	modulePath = cfg.Path
	children, err := os.ReadDir(modulePath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to open alloy config dir: %w", err)
	}

	files = make(map[string][]byte, len(children))
	for _, ch := range children {
		if ch.IsDir() {
			continue
		}

		if !strings.HasSuffix(ch.Name(), ".alloy") {
			continue
		}

		fpath := filepath.Join(modulePath, ch.Name())
		data, err := os.ReadFile(fpath)
		if err != nil {
			return "", nil, err
		}

		if err := validateAlloyConfig(fpath, data); err != nil {
			return "", nil, fmt.Errorf("error in Alloy config file %q: %w", fpath, err)
		}

		files[fpath] = data
	}

	return modulePath, files, nil
}

// validateAlloyConfig checks whether Alloy config contains statements unsupported in extension mode.
func validateAlloyConfig(fname string, data []byte) error {
	tree, err := parser.ParseFile(fname, data)
	if err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// TODO: throw warning if 'import.file' uses 'module_path' in inline config.
	for _, stmt := range tree.Body {
		block, ok := stmt.(*ast.BlockStmt)
		if !ok {
			continue
		}

		blockName := block.GetBlockName()
		if blockName == remotecfg.ServiceName {
			return fmt.Errorf("block %q is not supported in Alloy extension", blockName)
		}
	}

	return nil
}

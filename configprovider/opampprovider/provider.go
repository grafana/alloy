package opampprovider // import "github.com/grafana/alloy/configprovider/opampprovider"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
)

const schemeName = "opamp"

type provider struct {
	logger *zap.Logger
}

func NewFactory() confmap.ProviderFactory {
	return confmap.NewProviderFactory(newProvider)
}

func newProvider(ps confmap.ProviderSettings) confmap.Provider {
	log := ps.Logger
	if log == nil {
		log = zap.NewNop()
	}
	return &provider{logger: log}
}

func (*provider) Scheme() string {
	return schemeName
}

func (*provider) Shutdown(context.Context) error {
	return nil
}

func (p *provider) Retrieve(ctx context.Context, uri string, watcher confmap.WatcherFunc) (*confmap.Retrieved, error) {
	if !strings.HasPrefix(uri, schemeName+":") {
		return nil, fmt.Errorf("%q uri is not supported by %q provider", uri, schemeName)
	}
	basePath := strings.TrimPrefix(uri, schemeName+":")
	basePath = filepath.Clean(basePath)

	baseBytes, root, err := readBootstrapYAML(basePath)
	if err != nil {
		return nil, err
	}
	remoteDir, err := opampRemoteDirectory(root, basePath)
	if err != nil {
		return nil, err
	}
	merged, err := mergedConfigWithRemoteDir(baseBytes, remoteDir)
	if err != nil {
		return nil, err
	}

	if watcher == nil {
		return confmap.NewRetrieved(merged)
	}

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("fsnotify: %w", err)
	}
	if err := fsw.Add(remoteDir); err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("watch remote dir %s: %w", remoteDir, err)
	}
	if err := fsw.Add(basePath); err != nil {
		_ = fsw.Close()
		return nil, fmt.Errorf("watch bootstrap %s: %w", basePath, err)
	}

	var debounceMu sync.Mutex
	var debounceTimer *time.Timer
	run := func() {
		select {
		case <-stopCh:
			return
		default:
		}
		baseBytes, root, err := readBootstrapYAML(basePath)
		if err != nil {
			p.logger.Error("opamp provider: read bootstrap after watch event", zap.Error(err))
			return
		}
		remoteDir, err := opampRemoteDirectory(root, basePath)
		if err != nil {
			p.logger.Error("opamp provider: remote directory after watch event", zap.Error(err))
			return
		}
		if _, err := mergedConfigWithRemoteDir(baseBytes, remoteDir); err != nil {
			p.logger.Error("opamp provider: merge after watch event", zap.Error(err))
			return
		}
		watcher(&confmap.ChangeEvent{})
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopCh:
				return
			case err, ok := <-fsw.Errors:
				if !ok {
					return
				}
				if err != nil {
					p.logger.Warn("opamp provider: fsnotify error", zap.Error(err))
				}
			case _, ok := <-fsw.Events:
				if !ok {
					return
				}
				debounceMu.Lock()
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(200*time.Millisecond, run)
				debounceMu.Unlock()
			}
		}
	}()

	closeFn := func(_ context.Context) error {
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceMu.Unlock()
		close(stopCh)
		_ = fsw.Close()
		wg.Wait()
		return nil
	}

	return confmap.NewRetrieved(merged, confmap.WithRetrievedClose(closeFn))
}

// mergedConfigWithRemoteDir merges remote-directory YAML into the bootstrap YAML bytes and returns the combined map.
func mergedConfigWithRemoteDir(baseBytes []byte, remoteDir string) (map[string]any, error) {
	matches, err := sortedRemoteYAMLPaths(remoteDir)
	if err != nil {
		return nil, err
	}

	mergedRemote, err := mergeYAMLFilesSorted(matches)
	if err != nil {
		return nil, err
	}
	remoteConf, err := mergedRemote.AsConf()
	if err != nil {
		return nil, err
	}
	if err := mergedRemote.Close(context.Background()); err != nil {
		return nil, err
	}

	baseRet, err := confmap.NewRetrievedFromYAML(baseBytes)
	if err != nil {
		return nil, fmt.Errorf("base yaml: %w", err)
	}
	baseConf, err := baseRet.AsConf()
	if err != nil {
		_ = baseRet.Close(context.Background())
		return nil, err
	}
	if err := baseRet.Close(context.Background()); err != nil {
		return nil, err
	}

	if err := baseConf.Merge(remoteConf); err != nil {
		return nil, fmt.Errorf("merge remote dir into base: %w", err)
	}

	return baseConf.ToStringMap(), nil
}

func remoteConfigurationDirectory(root map[string]any) (string, error) {
	extVal, ok := root["extensions"]
	if !ok {
		return "", fmt.Errorf("missing required key extensions.opamp.remote_configuration_directory")
	}
	ext, ok := extVal.(map[string]any)
	if !ok {
		return "", fmt.Errorf("extensions must be a map")
	}
	opampVal, ok := ext["opamp"]
	if !ok {
		return "", fmt.Errorf("missing required key extensions.opamp.remote_configuration_directory")
	}
	opamp, ok := opampVal.(map[string]any)
	if !ok {
		return "", fmt.Errorf("extensions.opamp must be a map")
	}
	dirVal, ok := opamp["remote_configuration_directory"]
	if !ok {
		return "", fmt.Errorf("missing required key extensions.opamp.remote_configuration_directory")
	}
	dir, ok := dirVal.(string)
	if !ok || strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("extensions.opamp.remote_configuration_directory must be a non-empty string")
	}
	return dir, nil
}

// sortedRemoteYAMLPaths returns sorted paths matching *.yaml and *.yml under remoteDir.
func sortedRemoteYAMLPaths(remoteDir string) ([]string, error) {
	var paths []string
	for _, pattern := range []string{filepath.Join(remoteDir, "*.yaml"), filepath.Join(remoteDir, "*.yml")} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}
		paths = append(paths, matches...)
	}
	sort.Strings(paths)
	return paths, nil
}

// MergeRemoteYAMLFilesInDir reads and merges all *.yaml and *.yml files under remoteDir
func MergeRemoteYAMLFilesInDir(remoteDir string) (*confmap.Conf, error) {
	paths, err := sortedRemoteYAMLPaths(remoteDir)
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return confmap.New(), nil
	}
	retrieved, err := mergeYAMLFilesSorted(paths)
	if err != nil {
		return nil, err
	}
	conf, err := retrieved.AsConf()
	if err != nil {
		_ = retrieved.Close(context.Background())
		return nil, err
	}
	if err := retrieved.Close(context.Background()); err != nil {
		return nil, err
	}
	return conf, nil
}

func mergeYAMLFilesSorted(paths []string) (*confmap.Retrieved, error) {
	sort.Strings(paths)

	cfg := confmap.New()
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		ret, err := confmap.NewRetrievedFromYAML(content)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		next, err := ret.AsConf()
		if err != nil {
			_ = ret.Close(context.Background())
			return nil, fmt.Errorf("conf %s: %w", path, err)
		}
		if err := cfg.Merge(next); err != nil {
			_ = ret.Close(context.Background())
			return nil, fmt.Errorf("merge %s: %w", path, err)
		}
		if err := ret.Close(context.Background()); err != nil {
			return nil, err
		}
	}
	return confmap.NewRetrieved(cfg.ToStringMap())
}

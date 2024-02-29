package agentctl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/agent/internal/static/client"
	"github.com/grafana/agent/internal/static/metrics/instance"
)

// ConfigSync loads YAML files from a directory and syncs them to the
// provided PrometheusClient API. All YAML files will be synced and
// must be valid.
//
// The base name of the YAML file (i.e., without the file extension)
// is used as the config name.
//
// ConfigSync will completely overwrite the set of active configs
// present in the provided PrometheusClient - configs present in the
// API but not in the directory will be deleted.
func ConfigSync(logger log.Logger, cli client.PrometheusClient, dir string, dryRun bool) error {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	ctx := context.Background()
	cfgs, err := ConfigsFromDirectory(dir)
	if err != nil {
		return err
	}

	if dryRun {
		level.Info(logger).Log("msg", "config files validated successfully")
		return nil
	}

	uploaded := make(map[string]struct{}, len(cfgs))
	var hadErrors bool

	for _, cfg := range cfgs {
		level.Info(logger).Log("msg", "uploading config", "name", cfg.Name)
		err := cli.PutConfiguration(ctx, cfg.Name, cfg)
		if err != nil {
			level.Error(logger).Log("msg", "failed to upload config", "name", cfg.Name, "err", err)
			hadErrors = true
		}
		uploaded[cfg.Name] = struct{}{}
	}

	existing, err := cli.ListConfigs(ctx)
	if err != nil {
		return fmt.Errorf("could not list configs: %w", err)
	}

	// Delete configs from the existing API list that we didn't upload.
	for _, existing := range existing.Configs {
		if _, existsLocally := uploaded[existing]; !existsLocally {
			level.Info(logger).Log("msg", "deleting config", "name", existing)
			err := cli.DeleteConfiguration(ctx, existing)
			if err != nil {
				level.Error(logger).Log("msg", "failed to delete outdated config", "name", existing, "err", err)
				hadErrors = true
			}
		}
	}

	if hadErrors {
		return errors.New("one or more configurations failed to be modified; check the logs for more details")
	}

	return nil
}

// ConfigsFromDirectory parses all YAML files from a directory and
// loads each as an instance.Config.
func ConfigsFromDirectory(dir string) ([]*instance.Config, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if dir == path {
				return nil
			}
			return filepath.SkipDir
		}

		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var configs []*instance.Config
	for _, file := range files {
		cfg, err := configFromFile(file)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}

	return configs, nil
}

func configFromFile(path string) (*instance.Config, error) {
	var (
		fileName   = filepath.Base(path)
		configName = strings.TrimSuffix(fileName, filepath.Ext(fileName))
	)

	f, err := os.Open(path)
	if f != nil {
		defer f.Close()
	}
	if err != nil {
		return nil, err
	}

	cfg, err := instance.UnmarshalConfig(f)
	if err != nil {
		return nil, err
	}
	cfg.Name = configName
	return cfg, nil
}

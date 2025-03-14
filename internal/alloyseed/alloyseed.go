package alloyseed

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/grafana/alloy/internal/service/logging/level"
	"github.com/prometheus/common/version"
)

// Seed identifies a unique Alloy instance.
type Seed struct {
	UID       string    `json:"UID"`
	CreatedAt time.Time `json:"created_at"`
	Version   string    `json:"version"`
}

const (
	// Both LegacyHeaderName and HeaderName should be used to identify the Alloy
	// instance in the headers of requests. LegacyHeaderName is used for
	// backwards compatibility.

	LegacyHeaderName = "X-Agent-Id" // LegacyHeaderName represents the header name used prior to the Alloy release.
	HeaderName       = "X-Alloy-Id" // HeaderName represents the ID header to use for Alloy.
)

const filename = "alloy_seed.json"

var savedSeed *Seed
var once sync.Once

// Init should be called by an app entrypoint as soon as it can to configure
// where the unique seed will be stored. dir is the directory where we will
// read and store alloy_seed.json If left empty it will default to $APPDATA or
// /tmp A unique Alloy seed will be generated when this method is first called,
// and reused for the lifetime of this Alloy instance.
func Init(dir string, l log.Logger) {
	if l == nil {
		l = log.NewNopLogger()
	}
	once.Do(func() {
		loadOrGenerate(dir, l)
	})
}

func loadOrGenerate(dir string, l log.Logger) {
	var err error
	var seed *Seed
	// list of paths in preference order.
	// we will always write to the first path
	paths := []string{}
	if dir != "" {
		paths = append(paths, filepath.Join(dir, filename))
	}
	paths = append(paths, legacyPath())
	defer func() {
		// as a fallback, gen and save a new uid
		if seed == nil || seed.UID == "" {
			seed = generateNew()
			writeSeedFile(seed, paths[0], l)
		}
		// Finally save seed
		savedSeed = seed
	}()
	for i, p := range paths {
		if fileExists(p) {
			if seed, err = readSeedFile(p, l); err == nil {
				if i == 0 {
					// we found it at the preferred path. Just return it
					return
				} else {
					// it was at a backup path. write it to the preferred path.
					writeSeedFile(seed, paths[0], l)
					return
				}
			}
		}
	}
}

func generateNew() *Seed {
	return &Seed{
		UID:       uuid.NewString(),
		Version:   version.Version,
		CreatedAt: time.Now(),
	}
}

// Get returns a unique seed for this Alloy instance.
// It will always return a valid seed, even if previous attempts to
// load or save the seed file have failed
func Get() *Seed {
	// Init should have been called before this. If not, call it now with defaults.
	once.Do(func() {
		loadOrGenerate("", log.NewNopLogger())
	})
	if savedSeed != nil {
		return savedSeed
	}
	// we should never get here. But if somehow we do,
	// still return a valid seed for this request only
	return generateNew()
}

// readSeedFile reads the Alloy seed file
func readSeedFile(path string, logger log.Logger) (*Seed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		level.Error(logger).Log("msg", "Reading seed file", "err", err)
		return nil, err
	}
	seed := &Seed{}
	err = json.Unmarshal(data, seed)
	if err != nil {
		level.Error(logger).Log("msg", "Decoding seed file", "err", err)
		return nil, err
	}

	if seed.UID == "" {
		level.Error(logger).Log("msg", "Seed file has empty uid")
	}
	return seed, nil
}

func legacyPath() string {
	// windows
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), filename)
	}
	// linux/mac
	return filepath.Join("/tmp", filename)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, os.ErrNotExist)
}

// writeSeedFile writes the Alloy seed file
func writeSeedFile(seed *Seed, path string, logger log.Logger) {
	data, err := json.Marshal(*seed)
	if err != nil {
		level.Error(logger).Log("msg", "Encoding seed file", "err", err)
		return
	}
	err = os.WriteFile(path, data, 0644)
	if err != nil {
		level.Error(logger).Log("msg", "Writing seed file", "err", err)
		return
	}
}

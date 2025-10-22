package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	yaml "gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	positionFileMode = 0600
	cursorKeyPrefix  = "cursor-"
	journalKeyPrefix = "journal-"
)

// Config describes where to get position information from.
type Config struct {
	SyncPeriod        time.Duration `mapstructure:"sync_period" yaml:"sync_period"`
	PositionsFile     string        `mapstructure:"filename" yaml:"filename"`
	IgnoreInvalidYaml bool          `mapstructure:"ignore_invalid_yaml" yaml:"ignore_invalid_yaml"`
	ReadOnly          bool          `mapstructure:"-" yaml:"-"`
}

// RegisterFlagsWithPrefix registers flags where every name is prefixed by
// prefix. If prefix is a non-empty string, prefix should end with a period.
func (cfg *Config) RegisterFlagsWithPrefix(prefix string, f *flag.FlagSet) {
	f.DurationVar(&cfg.SyncPeriod, prefix+"positions.sync-period", 10*time.Second, "Period with this to sync the position file.")
	f.StringVar(&cfg.PositionsFile, prefix+"positions.file", "/var/log/positions.yaml", "Location to read/write positions from.")
	f.BoolVar(&cfg.IgnoreInvalidYaml, prefix+"positions.ignore-invalid-yaml", false, "whether to ignore & later overwrite positions files that are corrupted")
}

// RegisterFlags register flags.
func (cfg *Config) RegisterFlags(flags *flag.FlagSet) {
	cfg.RegisterFlagsWithPrefix("", flags)
}

// Positions tracks how far through each file we've read.
type positions struct {
	logger    log.Logger
	cfg       Config
	mtx       sync.Mutex
	positions map[Entry]string
	quit      chan struct{}
	done      chan struct{}
}

// Entry describes a positions file entry consisting of an absolute file path and
// the matching label set.
// An entry expects the string representation of a LabelSet or a Labels slice
// so that it can be utilized as a YAML key. The caller should make sure that
// the order and structure of the passed string representation is reproducible,
// and maintains the same format for both reading and writing from/to the
// positions file.
type Entry struct {
	Path   string `yaml:"path"`
	Labels string `yaml:"labels"`
}

// File format for the positions data.
type File struct {
	Positions map[Entry]string `yaml:"positions"`
}

type Positions interface {
	// GetString returns how far we've through a file as a string.
	// JournalTarget writes a journal cursor to the positions file, while
	// FileTarget writes an integer offset. Use Get to read the integer
	// offset.
	GetString(path, labels string) string
	// Get returns how far we've read through a file. Returns an error
	// if the value stored for the file is not an integer.
	Get(path, labels string) (int64, error)
	// PutString records (asynchronously) how far we've read through a file.
	// Unlike Put, it records a string offset and is only useful for
	// JournalTargets which doesn't have integer offsets.
	PutString(path, labels string, pos string)
	// Put records (asynchronously) how far we've read through a file.
	Put(path, labels string, pos int64)
	// Remove removes the position tracking for a filepath
	Remove(path, labels string)
	// SyncPeriod returns how often the positions file gets resynced
	SyncPeriod() time.Duration
	// Stop the Position tracker.
	Stop()
}

// LegacyFile is the copied struct for the static mode positions file.
type LegacyFile struct {
	Positions map[string]string `yaml:"positions"`
}

// ConvertLegacyPositionsFile will convert the legacy positions file to the new format if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
func ConvertLegacyPositionsFile(legacyPath, newPath string, l log.Logger) {
	legacyPositions := readLegacyFile(legacyPath, l)
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		level.Info(l).Log("msg", "will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		level.Info(l).Log("msg", "will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return
	}

	newPositions := make(map[Entry]string)
	for k, v := range legacyPositions.Positions {
		newPositions[Entry{
			Path: k,
			// This is a map of labels but must be an empty map since that is what the new positions expects.
			Labels: "{}",
		}] = v
	}
	err = writePositionFile(newPath, newPositions)
	if err != nil {
		level.Error(l).Log("msg", "error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	level.Info(l).Log("msg", "successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
}

// ConvertLegacyPositionsFileJournal will convert the legacy positions file to the new format for a journal job if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
//
// legacyJob is the name of the journal job in e.g. promatil or agent static.
func ConvertLegacyPositionsFileJournal(legacyPath, legacyJob string, newPath string, componentID string, l log.Logger) {
	legacyPositions := readLegacyFile(legacyPath, l)
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		level.Info(l).Log("msg", "will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		level.Info(l).Log("msg", "will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return
	}

	var (
		legacyCursor = CursorKey(legacyJob)
		newCursor    = CursorKey(componentID)
	)

	newPositions := make(map[Entry]string)
	for k, v := range legacyPositions.Positions {
		if k == legacyCursor {
			newPositions[Entry{
				Path:   newCursor,
				Labels: "{}",
			}] = v
			break
		}
	}
	err = writePositionFile(newPath, newPositions)
	if err != nil {
		level.Error(l).Log("msg", "error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	level.Info(l).Log("msg", "successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
}

func readLegacyFile(legacyPath string, l log.Logger) *LegacyFile {
	oldFile, err := os.Stat(legacyPath)
	// If the old file doesn't exist or is empty then return early.
	if err != nil || oldFile.Size() == 0 {
		level.Info(l).Log("msg", "no legacy positions file found", "path", legacyPath)
		return nil
	}
	// Try to read and parse the legacy file.
	clean := filepath.Clean(legacyPath)
	buf, err := os.ReadFile(clean)
	if err != nil {
		level.Error(l).Log("msg", "error reading legacy positions file", "path", clean, "error", err)
		return nil
	}
	legacyPositions := &LegacyFile{}
	err = yaml.UnmarshalStrict(buf, legacyPositions)
	if err != nil {
		level.Error(l).Log("msg", "error parsing legacy positions file", "path", clean, "error", err)
		return nil
	}
	return legacyPositions
}

// New makes a new Positions.
func New(logger log.Logger, cfg Config) (Positions, error) {
	positionData, err := readPositionsFile(cfg, logger)
	if err != nil {
		return nil, err
	}

	p := &positions{
		logger:    logger,
		cfg:       cfg,
		positions: positionData,
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
	}

	go p.run()
	return p, nil
}

func (p *positions) Stop() {
	close(p.quit)
	<-p.done
}

func (p *positions) PutString(path, labels string, pos string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.positions[Entry{path, labels}] = pos
}

func (p *positions) Put(path, labels string, pos int64) {
	p.PutString(path, labels, strconv.FormatInt(pos, 10))
}

func (p *positions) GetString(path, labels string) string {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.positions[Entry{path, labels}]
}

func (p *positions) Get(path, labels string) (int64, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	pos, ok := p.positions[Entry{path, labels}]
	if !ok {
		return 0, nil
	}

	return strconv.ParseInt(pos, 10, 64)
}

func (p *positions) Remove(path, labels string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.remove(path, labels)
}

func (p *positions) remove(path, labels string) {
	delete(p.positions, Entry{path, labels})
}

func (p *positions) SyncPeriod() time.Duration {
	return p.cfg.SyncPeriod
}

func (p *positions) run() {
	defer func() {
		p.save()
		level.Debug(p.logger).Log("msg", "positions saved")
		close(p.done)
	}()

	ticker := time.NewTicker(p.cfg.SyncPeriod)
	for {
		select {
		case <-p.quit:
			return
		case <-ticker.C:
			p.save()
			p.cleanup()
		}
	}
}

func (p *positions) save() {
	if p.cfg.ReadOnly {
		return
	}
	p.mtx.Lock()
	positions := make(map[Entry]string, len(p.positions))
	for k, v := range p.positions {
		positions[k] = v
	}
	p.mtx.Unlock()

	if err := writePositionFile(p.cfg.PositionsFile, positions); err != nil {
		level.Error(p.logger).Log("msg", "error writing positions file", "error", err)
	}
}

// CursorKey returns a key that can be saved as a cursor that is never deleted.
func CursorKey(key string) string {
	return fmt.Sprintf("%s%s", cursorKeyPrefix, key)
}

func (p *positions) cleanup() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	toRemove := []Entry{}
	for k := range p.positions {
		// If the position file is prefixed with cursor, it's a
		// cursor and not a file on disk.
		// We still have to support journal files, so we keep the previous check to avoid breaking change.
		if strings.HasPrefix(k.Path, cursorKeyPrefix) || strings.HasPrefix(k.Path, journalKeyPrefix) {
			continue
		}

		if _, err := os.Stat(k.Path); err != nil {
			if os.IsNotExist(err) {
				// File no longer exists.
				toRemove = append(toRemove, k)
			} else {
				// Can't determine if file exists or not, some other error.
				level.Warn(p.logger).Log("msg", "could not determine if log file "+
					"still exists while cleaning positions file", "error", err)
			}
		}
	}
	for _, tr := range toRemove {
		p.remove(tr.Path, tr.Labels)
	}
}

func readPositionsFile(cfg Config, logger log.Logger) (map[Entry]string, error) {
	cleanfn := filepath.Clean(cfg.PositionsFile)
	buf, err := os.ReadFile(cleanfn)
	if err != nil {
		if os.IsNotExist(err) {
			return map[Entry]string{}, nil
		}
		return nil, err
	}

	var p File
	err = yaml.UnmarshalStrict(buf, &p)
	if err != nil {
		// return empty if cfg option enabled
		if cfg.IgnoreInvalidYaml {
			level.Debug(logger).Log("msg", "ignoring invalid positions file", "file", cleanfn, "error", err)
			return map[Entry]string{}, nil
		}

		return nil, fmt.Errorf("invalid yaml positions file [%s]: %v", cleanfn, err)
	}

	// p.Positions will be nil if the file exists but is empty
	if p.Positions == nil {
		p.Positions = map[Entry]string{}
	}

	return p.Positions, nil
}

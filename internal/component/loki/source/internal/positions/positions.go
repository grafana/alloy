package positions

import (
	"encoding"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax"
	"gopkg.in/yaml.v2"
)

type Positions interface {
	Update(cfg Config)
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

const (
	cursorKeyPrefix  = "cursor-"
	journalKeyPrefix = "journal-"
)

// CursorKey returns a key that can be saved as a cursor that is never deleted.
func CursorKey(key string) string {
	return cursorKeyPrefix + key
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

// File is the format for the positions data on disk.
type File struct {
	Positions map[Entry]string `yaml:"positions"`
}

var (
	_ syntax.Defaulter = (*Config)(nil)
	_ syntax.Validator = (*Config)(nil)
)

// Config describes where to get position information from.
type Config struct {
	KeyMode    KeyMode       `alloy:"key_mode,attr,optional"`
	SyncPeriod time.Duration `alloy:"sync_period,attr,optional"`
}

func (c *Config) Validate() error {
	if c.SyncPeriod <= 0 {
		return errors.New("sync_period must be greater than 0")
	}
	return nil
}

func (c *Config) SetToDefault() {
	c.KeyMode = KeyModeIncludeLabels
	c.SyncPeriod = 10 * time.Second
}

var (
	_ encoding.TextUnmarshaler = (*KeyMode)(nil)
	_ encoding.TextMarshaler   = (KeyMode)("")
)

type KeyMode string

const (
	KeyModeIncludeLabels KeyMode = "include_labels"
	KeyModeExcludeLabels KeyMode = "exclude_labels"
)

// UnmarshalText implements encoding.TextUnmarshaler.
func (m *KeyMode) UnmarshalText(text []byte) error {
	s := KeyMode(text)
	switch s {
	case KeyModeIncludeLabels, KeyModeExcludeLabels:
		*m = s
	default:
		return fmt.Errorf("unknown key_mode value: %s", s)
	}
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (k KeyMode) MarshalText() (text []byte, err error) {
	return []byte(k), nil
}

// PositionsFile tracks how far through each file we've read.
type PositionsFile struct {
	logger log.Logger
	path   string

	// posMut is used to protect shared access to positions.
	// If we need to lock both posMut and cfgMut we must take
	// posMut first.
	posMut    sync.RWMutex
	positions map[Entry]string

	// cfgMut is used to protect shared access to cfg.
	// If we need to lock both posMut and cfgMut we must take
	// posMut first.
	cfgMut sync.RWMutex
	cfg    Config

	quit chan struct{}
	done chan struct{}
}

// New makes a new Positions.
func New(logger log.Logger, path string, cfg Config) (Positions, error) {
	positionData, err := readPositionsFile(path)
	if err != nil {
		return nil, err
	}

	if cfg.SyncPeriod <= 0 {
		cfg.SyncPeriod = 10 * time.Second
	}

	p := &PositionsFile{
		logger:    logger,
		cfg:       cfg,
		path:      path,
		positions: positionData,
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
	}

	go p.run()
	return p, nil
}

func (p *PositionsFile) Update(cfg Config) {
	p.cfgMut.RLock()
	if configChanged(p.cfg, cfg) {
		p.cfgMut.RUnlock()
		p.cfgMut.Lock()
		defer p.cfgMut.Unlock()
		if cfg.SyncPeriod <= 0 {
			cfg.SyncPeriod = 10 * time.Second
		}
		p.cfg = cfg
		return
	}
	p.cfgMut.RUnlock()
}

func configChanged(prev, next Config) bool {
	return prev.SyncPeriod != next.SyncPeriod || prev.KeyMode != next.KeyMode
}

func (p *PositionsFile) Get(key, labels string) (int64, error) {
	p.posMut.RLock()
	defer p.posMut.RUnlock()
	str, ok := p.get(key, labels)
	if !ok {
		return 0, nil
	}
	return strconv.ParseInt(str, 10, 64)
}

func (p *PositionsFile) GetString(key, labels string) string {
	p.posMut.RLock()
	defer p.posMut.RUnlock()
	str, _ := p.get(key, labels)
	return str
}

func (p *PositionsFile) get(key, labels string) (string, bool) {
	p.cfgMut.RLock()
	defer p.cfgMut.RUnlock()

	if p.cfg.KeyMode == KeyModeExcludeLabels {
		// First we try to get position by key.
		pos, ok := p.positions[Entry{key, ""}]
		if !ok {
			// Fallback to position with key and labels.
			pos, ok = p.positions[Entry{key, labels}]
		}

		return pos, ok
	}

	// First we try to get position by key and labels.
	pos, ok := p.positions[Entry{key, labels}]
	if !ok {
		// Fallback to position without labels
		pos, ok = p.positions[Entry{key, ""}]
	}

	return pos, ok
}

func (p *PositionsFile) Put(key, labels string, pos int64) {
	p.posMut.Lock()
	defer p.posMut.Unlock()
	p.put(key, labels, strconv.FormatInt(pos, 10))
}

func (p *PositionsFile) PutString(key, labels, pos string) {
	p.posMut.Lock()
	defer p.posMut.Unlock()
	p.put(key, labels, pos)
}

func (p *PositionsFile) put(key, labels, pos string) {
	p.cfgMut.RLock()
	defer p.cfgMut.RUnlock()

	if p.cfg.KeyMode == KeyModeExcludeLabels {
		p.positions[Entry{key, ""}] = pos
		return
	}
	p.positions[Entry{key, labels}] = pos
}

func (p *PositionsFile) Remove(key, labels string) {
	p.posMut.Lock()
	defer p.posMut.Unlock()
	p.remove(key, labels)
}

func (p *PositionsFile) remove(key, labels string) {
	// NOTE: we remove entries both with and without key so we don't
	// have orphaned positions stored.
	delete(p.positions, Entry{key, ""})
	delete(p.positions, Entry{key, labels})
}

func (p *PositionsFile) SyncPeriod() time.Duration {
	p.cfgMut.RLock()
	defer p.cfgMut.RUnlock()
	return p.cfg.SyncPeriod
}

func (p *PositionsFile) Stop() {
	close(p.quit)
	<-p.done
}

func (p *PositionsFile) run() {
	defer func() {
		p.save()
		close(p.done)
	}()

	ticker := time.NewTicker(p.SyncPeriod())

	for {
		select {
		case <-p.quit:
			return
		case <-ticker.C:
			p.save()
			p.cleanup()
			ticker.Reset(p.SyncPeriod())
		}
	}
}

func (p *PositionsFile) save() {
	p.posMut.Lock()
	positions := make(map[Entry]string, len(p.positions))
	maps.Copy(positions, p.positions)
	p.posMut.Unlock()

	if err := writePositionFile(p.path, positions); err != nil {
		level.Error(p.logger).Log("msg", "error writing positions file", "error", err)
	}

	level.Debug(p.logger).Log("msg", "positions saved")
}

func (p *PositionsFile) cleanup() {
	p.posMut.Lock()
	defer p.posMut.Unlock()

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

func readPositionsFile(path string) (map[Entry]string, error) {
	cleanfn := filepath.Clean(path)
	buf, err := os.ReadFile(cleanfn)
	if err != nil {
		if os.IsNotExist(err) {
			return map[Entry]string{}, nil
		}
		return nil, err
	}

	var p File
	err = yaml.Unmarshal(buf, &p)
	if err != nil {
		return nil, fmt.Errorf("invalid yaml positions file [%s]: %v", cleanfn, err)
	}

	// p.Positions will be nil if the file exists but is empty
	if p.Positions == nil {
		p.Positions = map[Entry]string{}
	}

	return p.Positions, nil
}

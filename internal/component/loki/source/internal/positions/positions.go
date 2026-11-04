package positions

// This code is copied from Promtail. The positions package allows logging
// components to keep track of read file offsets on disk and continue from the
// same place in case of a restart.

import (
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v2"
)

const (
	syncPeriod = 10 * time.Second

	cursorKeyPrefix  = "cursor-"
	journalKeyPrefix = "journal-"
)

// Config describes where to get position information from.
type Config struct {
	Path string
}

// Positions tracks how far through each file we've read.
type positions struct {
	logger    *slog.Logger
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

// New makes a new Positions.
func New(logger *slog.Logger, cfg Config) (Positions, error) {
	positionData, err := readPositionsFile(cfg)
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
	return syncPeriod
}

func (p *positions) run() {
	defer func() {
		p.save()
		p.logger.Debug("positions saved")
		close(p.done)
	}()

	ticker := time.NewTicker(syncPeriod)
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
	p.mtx.Lock()
	positions := make(map[Entry]string, len(p.positions))
	maps.Copy(positions, p.positions)
	p.mtx.Unlock()

	if err := writePositionFile(p.cfg.Path, positions); err != nil {
		p.logger.Error("error writing positions file", "error", err)
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
				p.logger.Warn("could not determine if log file still exists while cleaning positions file", "error", err)
			}
		}
	}
	for _, tr := range toRemove {
		p.remove(tr.Path, tr.Labels)
	}
}

func readPositionsFile(cfg Config) (map[Entry]string, error) {
	cleanfn := filepath.Clean(cfg.Path)
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

package positions

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

	cursorKeyPrefix = "cursor-"
)

// CursorKey returns a key that can be saved as a cursor that is never deleted.
func CursorKey(key string) string {
	return cursorKeyPrefix + key
}

// Positions stores read positions keyed by path and labels.
type Positions interface {
	// GetString returns the stored position, or "" if there is none.
	GetString(path, labels string) string
	// Get returns the stored position as an integer, or 0 if there is none.
	// It returns an error if the stored value is not an integer.
	Get(path, labels string) (int64, error)
	// PutString stores a position that isn't an integer, such as a cursor.
	PutString(path, labels string, pos string)
	// Put stores an integer position, such as a file offset.
	Put(path, labels string, pos int64)
	// Remove deletes the stored position.
	Remove(path, labels string)
	// Stop stops syncing and persists positions one last time.
	Stop()
}

var _ Positions = (*File)(nil)

// File is a Positions backed by a file on disk. Positions are kept in memory
// and written to the file periodically and on Stop.
type File struct {
	logger    *slog.Logger
	cfg       Config
	mtx       sync.Mutex
	positions map[Entry]string
	quit      chan struct{}
	done      chan struct{}
}

// Entry is the key for a stored position: a file path or cursor key, and the
// string form of its labels. Callers must format labels the same way on
// every read and write.
type Entry struct {
	Path   string `yaml:"path"`
	Labels string `yaml:"labels"`
}

type Config struct {
	Path string
}

// New returns a File that reads and writes positions at cfg.Path.
func New(logger *slog.Logger, cfg Config) (*File, error) {
	positionData, err := readPositionsFile(cfg)
	if err != nil {
		return nil, err
	}

	p := &File{
		logger:    logger,
		cfg:       cfg,
		positions: positionData,
		quit:      make(chan struct{}),
		done:      make(chan struct{}),
	}

	go p.run()
	return p, nil
}

func (p *File) Stop() {
	close(p.quit)
	<-p.done
}

func (p *File) PutString(path, labels string, pos string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.positions[Entry{path, labels}] = pos
}

func (p *File) Put(path, labels string, pos int64) {
	p.PutString(path, labels, strconv.FormatInt(pos, 10))
}

func (p *File) GetString(path, labels string) string {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.positions[Entry{path, labels}]
}

func (p *File) Get(path, labels string) (int64, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	pos, ok := p.positions[Entry{path, labels}]
	if !ok {
		return 0, nil
	}
	return strconv.ParseInt(pos, 10, 64)
}

func (p *File) Remove(path, labels string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.remove(path, labels)
}

func (p *File) remove(path, labels string) {
	delete(p.positions, Entry{path, labels})
}

func (p *File) run() {
	defer func() {
		p.save()
		p.logger.Debug("positions saved")
		close(p.done)
	}()

	ticker := time.NewTicker(syncPeriod)
	defer ticker.Stop()

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

func (p *File) save() {
	p.mtx.Lock()
	positions := maps.Clone(p.positions)
	p.mtx.Unlock()

	if err := writePositionFile(p.cfg.Path, positions); err != nil {
		p.logger.Error("error writing positions file", "error", err)
	}
}

func (p *File) cleanup() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	toRemove := []Entry{}
	for k := range p.positions {
		// If the position file is prefixed with cursor, it's a
		// cursor and not a file on disk.
		if strings.HasPrefix(k.Path, cursorKeyPrefix) {
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

// positionFile is the on-disk format of the positions file.
type positionFile struct {
	Positions map[Entry]string `yaml:"positions"`
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

	var p positionFile
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

func writePositionFile(path string, positions map[Entry]string) error {
	buf, err := yaml.Marshal(positionFile{
		Positions: positions,
	})
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Clean(path), buf)
}

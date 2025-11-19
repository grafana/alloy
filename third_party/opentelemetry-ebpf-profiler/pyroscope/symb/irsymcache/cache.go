package irsymcache // import "go.opentelemetry.io/ebpf-profiler/pyroscope/symb/irsymcache"

import (
	"debug/elf"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	lru "github.com/elastic/go-freelru"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/reporter"

	"go.opentelemetry.io/ebpf-profiler/process"
)

var errUnknownFile = errors.New("unknown file")

type cachedMarker int

var cached cachedMarker = 1
var erroredMarker cachedMarker = 2

type Table interface {
	Lookup(addr uint64) (SourceInfo, error)
	Close()
}

type TableFactory interface {
	ConvertTable(src *os.File, dst *os.File) error
	OpenTable(path string) (Table, error)
	Name() string
}

func NewTableFactory() TableFactory {
	return TableTableFactory{}
}

var _ reporter.ExecutableReporter = (*Resolver)(nil)

type Resolver struct {
	logger   *logrus.Entry
	f        TableFactory
	cacheDir string
	cache    *lru.SyncedLRU[libpf.FileID, cachedMarker]
	jobs     chan convertJob
	wg       sync.WaitGroup

	mutex    sync.Mutex
	tables   map[libpf.FileID]Table
	shutdown chan struct{}
}

func (c *Resolver) ReportExecutable(md *reporter.ExecutableMetadata) {
	if !md.MappingFile.Valid() {
		return
	}
	m := md.MappingFile.Value()
	if c.ExecutableKnown(m.FileID) {
		return
	}
	_ = c.ObserveExecutable(m.FileID, md)
}

func (c *Resolver) Cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, table := range c.tables {
		table.Close()
	}
	clear(c.tables)
}

type convertJob struct {
	src *os.File
	dst *os.File

	result chan error
}

type Options struct {
	Path        string
	SizeEntries uint32
}

func NewFSCache(impl TableFactory, opt Options) (*Resolver, error) {
	l := logrus.WithField("component", "irsymtab")
	l.WithFields(logrus.Fields{
		"path": opt.Path,
		"size": opt.SizeEntries,
	}).Debug()

	shutdown := make(chan struct{})
	res := &Resolver{
		logger:   l,
		f:        impl,
		cacheDir: opt.Path,
		jobs:     make(chan convertJob, 1),
		shutdown: shutdown,
		tables:   make(map[libpf.FileID]Table),
	}
	res.cacheDir = filepath.Join(res.cacheDir, impl.Name())

	if err := os.MkdirAll(res.cacheDir, 0o700); err != nil {
		return nil, err
	}

	cache, err := lru.NewSynced[libpf.FileID, cachedMarker](
		opt.SizeEntries,
		func(id libpf.FileID,
		) uint32 {
			return id.Hash32()
		})
	cache.SetOnEvict(func(id libpf.FileID, marker cachedMarker) {
		if marker == erroredMarker {
			return
		}
		filePath := res.tableFilePath(id)
		l.WithFields(logrus.Fields{
			"file": filePath,
		}).Debug("symbcache evicting")
		if err = os.Remove(filePath); err != nil {
			l.Error(err)
		}
	})
	if err != nil {
		return nil, err
	}
	res.cache = cache

	err = filepath.Walk(res.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		filename := filepath.Base(path)
		id, err := FileIDFromStringNoQuotes(filename)
		if err != nil {
			return nil
		}
		id2 := id.StringNoQuotes()
		if filename != id2 {
			return nil
		}
		res.cache.Add(id, cached)
		return nil
	})
	if err != nil {
		return nil, err
	}

	res.wg.Add(1)
	go func() {
		defer res.wg.Done()
		convertLoop(res, shutdown)
	}()

	return res, nil
}

func convertLoop(res *Resolver, shutdown <-chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	for {
		select {
		case <-shutdown:
			for len(res.jobs) > 0 {
				job := <-res.jobs
				job.result <- res.convertSync(job.src, job.dst)
			}
			return
		case job := <-res.jobs:
			job.result <- res.convertSync(job.src, job.dst)
		}
	}
}

func (c *Resolver) ExecutableKnown(id libpf.FileID) bool {
	_, known := c.cache.Get(id)
	return known
}

func (c *Resolver) ObserveExecutable(fid libpf.FileID, md *reporter.ExecutableMetadata) error {
	if !md.MappingFile.Valid() {
		return fmt.Errorf("invalid mapping file")
	}
	if md.MappingFile.Value().FileName == process.VdsoPathName {
		c.cache.Add(fid, cached)
		return nil
	}
	pid := md.Process.PID()
	l := c.logger.WithFields(logrus.Fields{
		"fid": fid.StringNoQuotes(),
		"elf": md.MappingFile.Value().FileName.String(),
		"pid": pid,
	})
	t1 := time.Now()
	err := c.convert(l, fid, md)
	if err != nil {
		c.cache.Add(fid, erroredMarker)
		l = l.WithError(err).WithField("duration", time.Since(t1))
		if !errors.Is(err, syscall.ESRCH) && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, elf.ErrNoSymbols) {
			l.Error("conversion failed")
		} else {
			l.Debug("conversion failed")
		}
	} else {
		l.WithField("duration", time.Since(t1)).Debug("converted")
	}
	return err
}

func (c *Resolver) convert(
	l *logrus.Entry,
	fid libpf.FileID,
	md *reporter.ExecutableMetadata,
) error {
	var err error
	var dst *os.File
	var src *os.File

	tableFilePath := c.tableFilePath(fid)
	info, err := os.Stat(tableFilePath)
	if err == nil && info != nil {
		return nil
	}

	if md.DebuglinkFileName != "" {
		debuglinkFileName, _ := md.Process.ExtractAsFile(md.DebuglinkFileName)
		src, err = os.Open(debuglinkFileName)
		if err != nil {
			l.WithError(err).Debug("open debug file")
		} else {
			defer src.Close()
		}
	}
	if src == nil {
		mf, err := md.Process.OpenMappingFile(md.Mapping)
		if err != nil {
			return err
		}
		defer mf.Close()
		src, _ = mf.(*os.File)
	}
	if src == nil {
		return errors.New("failed to open elf os file")
	}

	dst, err = os.Create(tableFilePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	err = c.convertAsync(src, dst)

	if err != nil {
		_ = os.Remove(tableFilePath)
		return err
	}
	c.cache.Add(fid, cached)
	return nil
}

func (c *Resolver) convertAsync(src, dst *os.File) error {
	job := convertJob{src: src, dst: dst, result: make(chan error)}
	c.jobs <- job
	return <-job.result
}

func (c *Resolver) convertSync(src, dst *os.File) error {
	return c.f.ConvertTable(src, dst)
}

func (c *Resolver) tableFilePath(fid libpf.FileID) string {
	return filepath.Join(c.cacheDir, fid.StringNoQuotes())
}

func (c *Resolver) ResolveAddress(
	fid libpf.FileID,
	addr uint64,
) (SourceInfo, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	v, known := c.cache.Get(fid)

	if !known || v == erroredMarker {
		return SourceInfo{}, errUnknownFile
	}
	t, ok := c.tables[fid]
	if ok {
		return t.Lookup(addr)
	}
	path := c.tableFilePath(fid)
	t, err := c.f.OpenTable(path)
	if err != nil {
		_ = os.Remove(path)
		c.cache.Remove(fid)
		return SourceInfo{}, err
	}
	c.tables[fid] = t
	return t.Lookup(addr)
}

func (c *Resolver) Close() error {
	c.mutex.Lock()
	if c.shutdown != nil {
		close(c.shutdown)
		c.shutdown = nil
	}
	c.mutex.Unlock()

	c.wg.Wait()

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, table := range c.tables {
		table.Close()
	}
	clear(c.tables)
	return nil
}

func FileIDFromStringNoQuotes(s string) (libpf.FileID, error) {
	return libpf.FileIDFromString(s)
}

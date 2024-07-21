package filequeue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"golang.design/x/chann"
)

type queue struct {
	mut       sync.RWMutex
	directory string
	maxIndex  int
	logger    log.Logger
	ch        *chann.Chann[string]
}

func NewQueue(directory string, logger log.Logger) (Storage, error) {
	err := os.MkdirAll(directory, 0777)
	if err != nil {
		return nil, err
	}

	matches, _ := filepath.Glob(filepath.Join(directory, "*.committed"))
	ids := make([]int, len(matches))
	names := make([]string, 0)

	for i, x := range matches {
		id, err := strconv.Atoi(strings.ReplaceAll(filepath.Base(x), ".committed", ""))
		if err != nil {
			continue
		}
		names = append(names, x)
		ids[i] = id
	}
	sort.Ints(ids)
	var currentIndex int
	if len(ids) > 0 {
		currentIndex = ids[len(ids)-1]
	}
	q := &queue{
		directory: directory,
		maxIndex:  currentIndex,
		logger:    logger,
		ch:        chann.New[string](),
	}
	for _, id := range ids {
		q.ch.In() <- filepath.Join(directory, fmt.Sprintf("%d.committed", id))
	}
	return q, nil
}

// Add a committed file to the queue.
func (q *queue) Add(data []byte) (string, error) {
	q.mut.Lock()
	defer q.mut.Unlock()
	q.maxIndex++
	name := filepath.Join(q.directory, fmt.Sprintf("%d.committed", q.maxIndex))
	level.Debug(q.logger).Log("msg", "adding bytes", "len", len(data), "name", name)
	err := q.writeFile(name, data)
	// In is an unbounded queue.
	q.ch.In() <- name
	return name, err
}

func (q *queue) Next(ctx context.Context, enc []byte) ([]byte, string, error) {
	select {
	case name := <-q.ch.Out():
		buf, err := q.readFile(name, enc)
		level.Debug(q.logger).Log("msg", "reading bytes", "len", len(buf), "name", name)

		if err != nil {
			return nil, "", err
		}
		return buf, name, nil
	case <-ctx.Done():
		q.ch.Close()
		return nil, "", errors.New("context done")
	}
}

func (q *queue) Delete(name string) {
	_ = os.Remove(name)
}

func (q *queue) writeFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0644)
}

func (q *queue) readFile(name string, enc []byte) ([]byte, error) {
	bb, err := os.ReadFile(name)
	if err != nil {
		return enc, err
	}
	return bb, err
}

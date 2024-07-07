package filequeue

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type filequeue struct {
	mut       sync.RWMutex
	directory string
	maxindex  int
	name      string
}

func newFileQueue(directory string, name string) (*filequeue, error) {
	err := os.MkdirAll(directory, 0777)
	if err != nil {
		return nil, err
	}

	matches, _ := filepath.Glob(filepath.Join(directory, "*.committed"))
	ids := make([]int, len(matches))
	for i, x := range matches {
		id, err := strconv.Atoi(strings.ReplaceAll(filepath.Base(x), ".committed", ""))
		if err != nil {
			continue
		}
		ids[i] = id
	}
	sort.Ints(ids)
	currentindex := 0
	if len(ids) > 0 {
		currentindex = ids[len(ids)-1]
	}
	q := &filequeue{
		directory: directory,
		maxindex:  currentindex,
		name:      name,
	}
	return q, nil
}

// Add a committed file to the queue.
func (q *filequeue) Add(data []byte) (string, error) {
	q.mut.Lock()
	defer q.mut.Unlock()

	q.maxindex++
	name := filepath.Join(q.directory, fmt.Sprintf("%d.committed", q.maxindex))
	err := q.writeFile(name, data)
	return name, err
}

// Name is a unique name for this file queue.
func (q *filequeue) Name() string {
	q.mut.Lock()
	defer q.mut.Unlock()

	return q.name
}

// Next retrieves the next file. If there are no files it will return false.
func (q *filequeue) Next(enc []byte) ([]byte, string, bool, bool) {
	q.mut.Lock()
	defer q.mut.Unlock()

	matches, err := filepath.Glob(filepath.Join(q.directory, "*.committed"))
	if err != nil {
		return nil, "", false, false
	}
	if len(matches) == 0 {
		return nil, "", false, false
	}
	ids := make([]int, len(matches))
	for i, x := range matches {
		id, err := strconv.Atoi(strings.ReplaceAll(filepath.Base(x), ".committed", ""))
		if err != nil {
			continue
		}
		ids[i] = id
	}

	sort.Ints(ids)
	name := filepath.Join(q.directory, fmt.Sprintf("%d.committed", ids[0]))
	enc, err = q.readFile(name, enc)
	if err != nil {
		return nil, "", false, false
	}
	return enc, name, true, len(ids) > 1
}

func (q *filequeue) Delete(name string) {
	q.mut.Lock()
	defer q.mut.Unlock()

	_ = os.Remove(name)
}

func (q *filequeue) writeFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0644)
}

func (q *filequeue) readFile(name string, enc []byte) ([]byte, error) {
	bb, err := os.ReadFile(name)
	if err != nil {
		return enc, err
	}
	return bb, err
}

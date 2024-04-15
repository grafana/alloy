package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/golang/snappy"
)

var bufPool = sync.Pool{New: func() any {
	s := make([]byte, 0)
	return &s
}}

type filequeue struct {
	mut       sync.RWMutex
	directory string
	maxindex  int
}

func newFileQueue(directory string) (*filequeue, error) {
	err := os.MkdirAll(directory, 0777)
	if err != nil {
		return nil, err
	}
	clearUncommitted(directory)

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
	}
	return q, nil
}

// AddCommited an committed file to the queue.
func (q *filequeue) AddCommited(data []byte) (string, error) {
	q.mut.Lock()
	defer q.mut.Unlock()

	q.maxindex++
	name := filepath.Join(q.directory, fmt.Sprintf("%d.committed", q.maxindex))
	err := q.writeFile(name, data)
	return name, err
}

// AddUncommited an uncommitted file to the queue.
func (q *filequeue) AddUncommited(data []byte) (string, error) {
	q.mut.Lock()
	defer q.mut.Unlock()

	q.maxindex++
	name := filepath.Join(q.directory, fmt.Sprintf("%d.uncommitted", q.maxindex))
	err := q.writeFile(name, data)
	return name, err
}

func (q *filequeue) Commit(handles []string) error {
	q.mut.Lock()
	defer q.mut.Unlock()

	for _, h := range handles {
		newname := strings.Replace(h, "uncommitted", "committed", 1)
		//TODO add windows specific check here
		err := os.Rename(h, newname)
		if err != nil {
			return err
		}
	}
	return nil
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

	os.Remove(name)
}

func clearUncommitted(directory string) {
	matches, err := filepath.Glob(filepath.Join(directory, "*.uncommitted"))
	if err != nil {
		return
	}
	for _, x := range matches {
		_ = os.Remove(x)
	}
}
func (q *filequeue) writeFile(name string, data []byte) error {
	pntBuf := bufPool.Get().(*[]byte)
	buffer := *pntBuf
	defer bufPool.Put(&buffer)
	buffer = snappy.Encode(buffer, data)
	return os.WriteFile(name, buffer, 0644)
}

func (q *filequeue) readFile(name string, enc []byte) ([]byte, error) {
	bb, err := os.ReadFile(name)
	if err != nil {
		return enc, err
	}

	enc, err = snappy.Decode(enc, bb)
	if err != nil {
		return enc, err
	}
	return enc, nil
}

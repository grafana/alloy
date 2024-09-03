package filequeue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/vladopajic/go-actor/actor"
)

var _ actor.Worker = (*queue)(nil)
var _ types.FileStorage = (*queue)(nil)

// queue represents the on disk queue. This is an ordered list by id in the format: <id>.committed
// Each file contains a byte buffer and an optional metatdata map.
type queue struct {
	self      actor.Actor
	directory string
	maxIndex  int
	logger    log.Logger
	files     actor.Mailbox[types.Data]
	// Out is where to send data when pulled from queue, it is assumed that it will
	// block until ready for another file.
	out func(ctx context.Context, dh types.DataHandle)
	// existingFiles is the list of files found initially.
	existingsFiles []string
}

// NewQueue returns a implementation of FileStorage.
func NewQueue(directory string, out func(ctx context.Context, dh types.DataHandle), logger log.Logger) (types.FileStorage, error) {
	err := os.MkdirAll(directory, 0777)
	if err != nil {
		return nil, err
	}

	// We dont actually support uncommitted but I think its good to at least have some naming to avoid parsing random files
	// that get installed into the system.
	matches, _ := filepath.Glob(filepath.Join(directory, "*.committed"))
	ids := make([]int, len(matches))

	// Try and grab the id from each file.
	// 1.committed
	for i, x := range matches {
		id, err := strconv.Atoi(strings.ReplaceAll(filepath.Base(x), ".committed", ""))
		if err != nil {
			continue
		}
		ids[i] = id
	}
	sort.Ints(ids)
	var currentIndex int
	if len(ids) > 0 {
		currentIndex = ids[len(ids)-1]
	}
	q := &queue{
		directory:      directory,
		maxIndex:       currentIndex,
		logger:         logger,
		out:            out,
		files:          actor.NewMailbox[types.Data](),
		existingsFiles: make([]string, 0),
	}

	// Push the files that currently exist to the channel.
	for _, id := range ids {
		name := filepath.Join(directory, fmt.Sprintf("%d.committed", id))
		q.existingsFiles = append(q.existingsFiles, name)
	}
	return q, nil
}

func (q *queue) Start() {
	q.self = actor.Combine(actor.New(q), q.files).Build()
	q.self.Start()

}

func (q *queue) Stop() {
	q.self.Stop()
}

func (q *queue) Store(ctx context.Context, meta map[string]string, data []byte) error {
	return q.files.Send(ctx, types.Data{
		Meta: meta,
		Data: data,
	})
}

// get returns the data of the file or an error if something wrong went on.
func get(name string) (map[string]string, []byte, error) {
	defer deleteFile(name)
	buf, err := readFile(name)
	if err != nil {
		return nil, nil, err
	}
	r := &Record{}
	_, err = r.UnmarshalMsg(buf)
	if err != nil {
		return nil, nil, err
	}
	return r.Meta, r.Data, nil
}

func (q *queue) DoWork(ctx actor.Context) actor.WorkerStatus {
	// Queue up our existing items.
	for _, name := range q.existingsFiles {
		q.out(ctx, types.DataHandle{
			Name: name,
			Get: func() (map[string]string, []byte, error) {
				return get(name)
			},
		})
	}
	// We only want to process existing files once.
	q.existingsFiles = nil
	select {
	case <-ctx.Done():
		return actor.WorkerEnd
	case item, ok := <-q.files.ReceiveC():
		if !ok {
			return actor.WorkerEnd
		}
		name, err := q.add(item.Meta, item.Data)
		if err != nil {
			level.Error(q.logger).Log("msg", "error adding item", "err", err)
			return actor.WorkerContinue
		}
		q.out(ctx, types.DataHandle{
			Name: name,
			Get: func() (map[string]string, []byte, error) {
				return get(name)
			},
		})
		return actor.WorkerContinue
	}
}

// Add a committed file to the queue.
func (q *queue) add(meta map[string]string, data []byte) (string, error) {
	if meta == nil {
		meta = make(map[string]string)
	}
	q.maxIndex++
	name := filepath.Join(q.directory, fmt.Sprintf("%d.committed", q.maxIndex))
	// record wraps the data and metadata in one. This allows the consumer to take action based on the map.
	r := &Record{
		Meta: meta,
		Data: data,
	}
	// TODO @mattdurham reuse a buffer here.
	rBuf, err := r.MarshalMsg(nil)
	if err != nil {
		return "", err
	}
	err = q.writeFile(name, rBuf)
	if err != nil {
		return "", err
	}
	return name, err
}

func (q *queue) writeFile(name string, data []byte) error {
	return os.WriteFile(name, data, 0644)
}

func deleteFile(name string) {
	_ = os.Remove(name)
}
func readFile(name string) ([]byte, error) {
	bb, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	return bb, err
}

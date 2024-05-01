package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/grafana/alloy/internal/featuregate"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/alloy/internal/component"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.test.logs",
		Args:      Arguments{},
		Exports:   Exports{},
		Stability: featuregate.StabilityExperimental,
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return NewComponent(opts, args.(Arguments))
		},
	})
}

type Component struct {
	mut         sync.Mutex
	o           component.Options
	index       int
	files       []string
	args        Arguments
	argsChan    chan Arguments
	writeTicker *time.Ticker
	churnTicker *time.Ticker
}

func NewComponent(o component.Options, c Arguments) (*Component, error) {
	err := os.MkdirAll(o.DataPath, 0750)
	if err != nil {
		return nil, err
	}
	entries, _ := os.ReadDir(o.DataPath)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		_ = os.Remove(filepath.Join(o.DataPath, e.Name()))
	}
	comp := &Component{
		args:        c,
		index:       1,
		files:       make([]string, 0),
		writeTicker: time.NewTicker(c.WriteCadence),
		churnTicker: time.NewTicker(c.FileRefresh),
		argsChan:    make(chan Arguments),
		o:           o,
	}
	o.OnStateChange(Exports{Directory: o.DataPath})
	return comp, nil
}

func (c *Component) Run(ctx context.Context) error {
	defer c.writeTicker.Stop()
	defer c.churnTicker.Stop()
	// Create the initial set of files.
	c.createFiles()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.writeTicker.C:
			c.writeFiles()
		case <-c.churnTicker.C:
			c.churnFiles()
		case args := <-c.argsChan:
			c.args = args
			c.writeTicker.Reset(c.args.WriteCadence)
			c.churnTicker.Reset(c.args.FileRefresh)
		}
	}
}

func (c *Component) Update(args component.Arguments) error {
	c.argsChan <- args.(Arguments)
	return nil
}

func (c *Component) writeFiles() {
	c.mut.Lock()
	defer c.mut.Unlock()

	// TODO add error handling and figure out why some files are 0 bytes.
	for _, f := range c.files {
		bb := bytes.Buffer{}
		for i := 0; i <= c.args.WritesPerCadence; i++ {
			attributes := make(map[string]string)
			attributes["ts"] = time.Now().Format(time.RFC3339)
			msgLen := 0
			if c.args.MessageMaxLength == c.args.MessageMinLength {
				msgLen = c.args.MessageMinLength
			} else {
				msgLen = rand.Intn(c.args.MessageMaxLength-c.args.MessageMinLength) + c.args.MessageMinLength
			}
			attributes["msg"] = RandStringBytes(msgLen)
			for k, v := range c.args.Labels {
				attributes[k] = v
			}
			data, err := json.Marshal(attributes)
			if err != nil {
				continue
			}
			bb.Write(data)
			bb.WriteString("\n")
		}
		fh, err := os.OpenFile(f, os.O_APPEND|os.O_WRONLY|os.O_CREATE|os.O_SYNC, 0644)
		if err != nil {
			level.Error(c.o.Logger).Log("msg", "error opening file", "file", f, "err", err)
			_ = fh.Close()
			continue
		}
		_, err = fh.Write(bb.Bytes())
		if err != nil {
			level.Error(c.o.Logger).Log("msg", "error writing file", "file", f, "err", err)
		}
		_ = fh.Close()
	}
}

func (c *Component) createFiles() {
	for {
		if c.args.NumberOfFiles > len(c.files) {
			fullpath := filepath.Join(c.o.DataPath, strconv.Itoa(c.index)+".log")
			c.files = append(c.files, fullpath)
			c.index++
		} else if c.args.NumberOfFiles < len(c.files) {
			c.files = c.files[:c.args.NumberOfFiles]
		}
		if c.args.NumberOfFiles == len(c.files) {
			return
		}
	}
}

func (c *Component) churnFiles() {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.createFiles()

	churn := int(float64(c.args.NumberOfFiles) * c.args.FileChurnPercent)
	for i := 0; i < churn; i++ {
		candidate := rand.Intn(len(c.files))
		fullpath := filepath.Join(c.o.DataPath, strconv.Itoa(c.index)+".log")
		c.files = append(c.files, fullpath)
		c.index++
		c.files[candidate] = fullpath
	}
}

type Arguments struct {
	// WriteCadance is the interval at which it will write to a file.
	WriteCadence     time.Duration     `alloy:"write_cadence,attr,optional"`
	WritesPerCadence int               `alloy:"writes_per_cadence,attr,optional"`
	NumberOfFiles    int               `alloy:"number_of_files,attr,optional"`
	Labels           map[string]string `alloy:"labels,attr,optional"`
	MessageMaxLength int               `alloy:"message_max_length,attr,optional"`
	MessageMinLength int               `alloy:"message_min_length,attr,optional"`
	FileChurnPercent float64           `alloy:"file_churn_percent,attr,optional"`
	// FileRefresh is the interval at which it will stop writing to a number of files equal to churn percent and start new ones.
	FileRefresh time.Duration `alloy:"file_refresh,attr,optional"`
}

// SetToDefault implements alloy.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = DefaultArguments()
}

func DefaultArguments() Arguments {
	return Arguments{
		WriteCadence:     1 * time.Second,
		NumberOfFiles:    1,
		MessageMaxLength: 100,
		MessageMinLength: 10,
		FileChurnPercent: 0.1,
		FileRefresh:      1 * time.Minute,
		WritesPerCadence: 1,
	}
}

// Validate implements alloy.Validator.
func (r *Arguments) Validate() error {
	if r.NumberOfFiles <= 0 {
		return fmt.Errorf("number_of_files must be greater than 0")
	}
	if r.MessageMaxLength < r.MessageMinLength {
		return fmt.Errorf("message_max_length must be greater than or equal to message_min_length")
	}
	if r.FileChurnPercent < 0 || r.FileChurnPercent > 1 {
		return fmt.Errorf("file_churn_percent must be between 0 and 1")
	}
	if r.WriteCadence < 0 {
		return fmt.Errorf("write_cadence must be greater than 0")
	}
	if r.FileRefresh < 0 {
		return fmt.Errorf("file_refresh must be greater than 0")
	}
	return nil
}

type Exports struct {
	Directory string `alloy:"directory,attr"`
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

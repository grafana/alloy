//go:build mage

package main

import (
	"bytes"
	"fmt"
	"github.com/fatih/color"
	"github.com/magefile/mage/sh"
	"os"
	"strings"
	"sync"
)

func ExecNoEnv(name string, cmd string, args ...string) error {
	return Exec(name, nil, cmd, args...)
}

func Exec(name string, env map[string]string, cmd string, args ...string) error {
	fmt.Println(name + " starting")
	if len(env) > 0 {
		fmt.Println(mapToString(env))
	}
	fmt.Println(cmd + strings.Join(args, " "))
	stdOut := os.Stdout //newWriter(name, false)
	stdErr := os.Stderr //newWriter(name, true)
	_, err := sh.Exec(env, stdOut, stdErr, cmd, args...)
	if err != nil {
		fmt.Println(name+" error: ", err)
	}
	fmt.Println(name + " finished")

	return err
}

type iobuffer struct {
	out *iowriter
	err *iowriter
	buf *bytes.Buffer
}

func newIOBuffer(name string) *iobuffer {
	iob := &iobuffer{
		out: newWriter(name, false),
		err: newWriter(name, true),
		buf: &bytes.Buffer{},
	}
	return iob
}

type iowriter struct {
	mut     sync.Mutex
	name    string
	isError bool
	clr     *color.Color
	buffer  *bytes.Buffer
}

func newWriter(name string, isError bool) *iowriter {
	var clr *color.Color
	if isError {
		clr = color.New(color.FgRed)
	} else {
		clr = color.New(color.FgGreen)
	}
	return &iowriter{
		name:    name,
		clr:     clr,
		isError: isError,
	}
}

func (i iowriter) Write(p []byte) (n int, err error) {
	i.mut.Lock()
	defer i.mut.Unlock()

	defer func() {
		if err != nil {
			fmt.Println(err)
		}
	}()
	if i.isError {
		n, err = os.Stdout.WriteString(i.clr.Sprintf("[%s] ", i.name) + string(p) + "\n")
	} else {
		n, err = os.Stdout.WriteString(i.clr.Sprintf("[%s] ", i.name) + string(p) + "\n")
	}
	return n, nil
}

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/grafana/river/diag"
	"github.com/grafana/river/parser"
	"github.com/grafana/river/printer"
)

func main() {
	err := run()

	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		for _, diag := range diags {
			fmt.Fprintln(os.Stderr, diag)
		}
		os.Exit(1)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "error: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		write bool
	)

	fs := flag.NewFlagSet("riverfmt", flag.ExitOnError)
	fs.BoolVar(&write, "w", write, "write result to (source) file instead of stdout")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	args := fs.Args()
	switch len(args) {
	case 0:
		if write {
			return fmt.Errorf("cannot use -w with standard input")
		}
		return format("<stdin>", nil, os.Stdin, write)

	case 1:
		fi, err := os.Stat(args[0])
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return fmt.Errorf("cannot format a directory")
		}
		f, err := os.Open(args[0])
		if err != nil {
			return err
		}
		defer f.Close()
		return format(args[0], fi, f, write)

	default:
		return fmt.Errorf("can only format one file")
	}
}

func format(filename string, fi os.FileInfo, r io.Reader, write bool) error {
	bb, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile(filename, bb)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, f); err != nil {
		return err
	}

	// Add a newline at the end
	_, _ = buf.Write([]byte{'\n'})

	if !write {
		_, err := io.Copy(os.Stdout, &buf)
		return err
	}

	wf, err := os.OpenFile(filename, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, fi.Mode().Perm())
	if err != nil {
		return err
	}
	defer wf.Close()

	_, err = io.Copy(wf, &buf)
	return err
}

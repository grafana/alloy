// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Implements a command-line utility for compressing and decompressing zstpak files.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	zstpak "go.opentelemetry.io/ebpf-profiler/tools/zstpak/lib"
)

func tryMain() error {
	var compress, decompress bool
	var in, out string
	var chunkSize uint64

	flag.BoolVar(&compress, "c", false, "Compress data into zstpak format")
	flag.BoolVar(&decompress, "d", false, "Decompress data from zstpak format")
	flag.StringVar(&in, "i", "", "The input file path")
	flag.StringVar(&out, "o", "", "The output file path")
	flag.Uint64Var(&chunkSize, "chunk-size", 65536, "The chunk size to use")
	flag.Parse()

	if compress == decompress {
		return errors.New("must specify either `-c` or `-d`")
	}
	if in == "" {
		return errors.New("missing required argument `i`")
	}
	if out == "" {
		return errors.New("missing required argument `o`")
	}

	outputFile, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}

	switch {
	case compress:
		inputFile, err := os.Open(in)
		if err != nil {
			return fmt.Errorf("failed to open input file: %w", err)
		}

		if err = zstpak.CompressInto(inputFile, outputFile, chunkSize); err != nil {
			return fmt.Errorf("failed to compress file: %w", err)
		}
	case decompress:
		pak, err := zstpak.Open(in)
		if err != nil {
			return fmt.Errorf("failed to open zstpak file: %w", err)
		}

		buf := make([]byte, pak.UncompressedSize())
		if _, err = pak.ReadAt(buf, 0); err != nil {
			return fmt.Errorf("failed to read zstpak: %w", err)
		}

		if _, err = outputFile.Write(buf); err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}
	}

	return nil
}

func main() {
	if err := tryMain(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
	}
}

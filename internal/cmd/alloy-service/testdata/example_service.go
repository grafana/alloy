package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var listenAddr string
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&listenAddr, "listen-addr", "0.0.0.0:8080", "Address to listen for traffic on.")

	if err := fs.Parse(os.Args[1:]); err != nil {
		return err
	}

	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = lis.Close()
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/echo/stdout", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(os.Stdout, r.Body)
	})
	mux.HandleFunc("/echo/stderr", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(os.Stderr, r.Body)
	})
	mux.HandleFunc("/echo/response", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(w, r.Body)
	})
	mux.HandleFunc("/echo/env", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Join(os.Environ(), "\n")))
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var (
		wg  sync.WaitGroup
		srv = &http.Server{Handler: mux}
	)
	wg.Go(func() {
		_ = srv.Serve(lis)
	})

	<-ctx.Done()
	_ = srv.Shutdown(context.Background())
	wg.Wait()

	fmt.Fprintln(os.Stdout, "graceful shutdown")
	return nil
}

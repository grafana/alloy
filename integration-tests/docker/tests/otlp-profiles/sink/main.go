package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/pprofile/pprofileotlp"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/encoding/gzip"
)

const healthURL = "http://localhost:8080/health"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check whether the sink is healthy")
	flag.Parse()
	if *healthcheck {
		checkHealth()
		return
	}

	sink := &profilesServer{}
	go serveHTTP(sink)
	lis, err := net.Listen("tcp", ":4317")
	if err != nil {
		panic(err)
	}
	server := grpc.NewServer()
	pprofileotlp.RegisterGRPCServer(server, sink)
	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}

func serveHTTP(sink *profilesServer) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/profiles", func(w http.ResponseWriter, _ *http.Request) {
		sink.mut.Lock()
		defer sink.mut.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(sink.profiles); err != nil {
			panic(err)
		}
	})
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}

func checkHealth() {
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get(healthURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, resp.Status)
		os.Exit(1)
	}
}

type profilesServer struct {
	pprofileotlp.UnimplementedGRPCServer
	mut      sync.Mutex
	profiles []json.RawMessage
}

func (s *profilesServer) Export(_ context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	body, err := (&pprofile.JSONMarshaler{}).MarshalProfiles(req.Profiles())
	if err != nil {
		return pprofileotlp.NewExportResponse(), err
	}
	s.mut.Lock()
	defer s.mut.Unlock()
	s.profiles = append(s.profiles, body)
	return pprofileotlp.NewExportResponse(), nil
}

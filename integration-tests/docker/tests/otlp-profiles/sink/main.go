package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"time"

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

	var profileCount atomic.Int64
	go serveHTTP(&profileCount)

	lis, err := net.Listen("tcp", ":4317")
	if err != nil {
		panic(err)
	}

	server := grpc.NewServer()
	pprofileotlp.RegisterGRPCServer(server, &profilesServer{profileCount: &profileCount})
	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}

func serveHTTP(profileCount *atomic.Int64) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/count", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]int64{"count": profileCount.Load()}); err != nil {
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
	profileCount *atomic.Int64
}

func (s *profilesServer) Export(_ context.Context, req pprofileotlp.ExportRequest) (pprofileotlp.ExportResponse, error) {
	s.profileCount.Add(int64(req.Profiles().ProfileCount()))
	return pprofileotlp.NewExportResponse(), nil
}

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	gprofile "github.com/google/pprof/profile"
	pushv1 "github.com/grafana/pyroscope/api/gen/proto/go/push/v1"
	"github.com/grafana/pyroscope/api/gen/proto/go/push/v1/pushv1connect"
	typesv1 "github.com/grafana/pyroscope/api/gen/proto/go/types/v1"
)

var (
	proxyURL    = flag.String("proxy-url", "http://localhost:4041", "URL of the pyroscope.receive_http proxy")
	instances   = flag.Int("instances", 50, "Number of unique instances sending profiles")
	concurrent  = flag.Int("concurrent", 100, "Number of concurrent requests")
	duration    = flag.Duration("duration", 30*time.Second, "Test duration")
	profileSize = flag.Int("profile-size", 10000, "Number of allocations to create in heap profile")
)

type Result struct {
	Duration  time.Duration
	Error     error
	Instance  int
	RequestID string
}

func generateHeapProfile(allocations int) ([]byte, error) {
	f, err := os.Open("heap")
	if err != nil {
		panic(err)
	}
	p, err := gprofile.Parse(f)
	if err != nil {
		panic(err)
	}
	p.TimeNanos = time.Now().UnixNano()
	buffer := bytes.NewBuffer(nil)
	err = p.Write(buffer)
	if err != nil {

		panic(err)
	}
	return buffer.Bytes(), nil

}

func sendProfile(client pushv1connect.PusherServiceClient, instanceID int, profileData []byte) Result {
	start := time.Now()

	// Create labels similar to user's setup
	labels := []*typesv1.LabelPair{
		{Name: "__name__", Value: "process_cpu"},
		{Name: "service_name", Value: "foundationdb"},
		{Name: "hostname", Value: fmt.Sprintf("host-%d", instanceID)},
		{Name: "instance", Value: fmt.Sprintf("instance-%d", instanceID)},
		{Name: "env", Value: "loadtest"},
		{Name: "process_class", Value: "transaction"},
		{Name: "port", Value: "4519"},
		{Name: "pid", Value: fmt.Sprintf("%d", 10000+instanceID)},
	}

	reqId := fmt.Sprintf("%016x", rand.Uint64()) + fmt.Sprintf("%016x", rand.Uint64())
	req := &pushv1.PushRequest{
		Series: []*pushv1.RawProfileSeries{
			{
				Labels: labels,
				Samples: []*pushv1.RawSample{
					{
						ID:         reqId,
						RawProfile: profileData,
					},
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rr := connect.NewRequest(req)

	_, err := client.Push(ctx, rr)

	return Result{
		Duration:  time.Since(start),
		Error:     err,
		Instance:  instanceID,
		RequestID: reqId,
	}
}

func runLoadTest() {
	log.Printf("Starting load test:\n")
	log.Printf("  Proxy URL: %s\n", *proxyURL)
	log.Printf("  Instances: %d\n", *instances)
	log.Printf("  Concurrent requests: %d\n", *concurrent)
	log.Printf("  Duration: %v\n", *duration)
	log.Printf("  Profile size: %d allocations\n", *profileSize)
	log.Println(strings.Repeat("-", 50))

	// Generate a sample profile once
	profileData, err := generateHeapProfile(*profileSize)
	if err != nil {
		log.Fatalf("Failed to generate profile: %v", err)
	}
	log.Printf("Generated profile size: %d bytes\n", len(profileData))

	// Create HTTP client with connection pooling
	httpClient := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *concurrent,
			MaxIdleConnsPerHost: *concurrent,
			MaxConnsPerHost:     *concurrent,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 30 * time.Second,
	}

	// Create Connect client
	client := pushv1connect.NewPusherServiceClient(httpClient, *proxyURL)

	var (
		wg sync.WaitGroup
	)

	for i := 0; i < *instances; i++ {

		instanceID := i

		wg.Add(1)
		go func() {
			defer wg.Done()

			result := sendProfile(client, instanceID, profileData)
			fmt.Printf("res %+v\n", result)
		}()
	}

	wg.Wait()

	fmt.Println("\n" + strings.Repeat("=", 50))

}

func main() {
	flag.Parse()
	runLoadTest()
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	requestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests.",
	}, []string{"method", "path", "status"})

	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	requestsInFlight = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "http_requests_in_flight",
		Help: "Number of HTTP requests currently being processed.",
	})
)

func init() {
	// Go and process collectors are registered by the default registry automatically.
	prometheus.MustRegister(requestsTotal, requestDuration, requestsInFlight)
}

// --- Data types ---

type Product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type CartItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type CartAddRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type Order struct {
	ID    string     `json:"id"`
	Items []CartItem `json:"items"`
	Total float64    `json:"total"`
}

// --- DB mode: in-memory data store ---

var products = []Product{
	{ID: "1", Name: "Mechanical Keyboard", Price: 89.99},
	{ID: "2", Name: "Wireless Mouse", Price: 34.50},
	{ID: "3", Name: "USB-C Hub", Price: 49.99},
	{ID: "4", Name: "Monitor Stand", Price: 27.00},
	{ID: "5", Name: "Desk Lamp", Price: 42.50},
}

func dbHandler(logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		logger.Debug("fetching all products", "count", len(products))
		time.Sleep(time.Duration(rand.IntN(10)) * time.Millisecond)
		writeJSON(w, products)
	})

	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		logger.Debug("fetching product", "id", id)
		for _, p := range products {
			if p.ID == id {
				writeJSON(w, p)
				return
			}
		}
		logger.Warn("product not found", "id", id)
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}

// --- Catalog mode: business logic layer ---

func catalogHandler(logger *slog.Logger, dbAddr string) http.Handler {
	mux := http.NewServeMux()
	client := &http.Client{Timeout: 5 * time.Second}

	var (
		mu    sync.Mutex
		carts = make(map[string][]CartItem) // session -> items
	)

	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("listing products")
		resp, err := client.Get(dbAddr + "/products")
		if err != nil {
			logger.Warn("db call failed", "error", err)
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		w.Write(buf.Bytes())
	})

	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		logger.Info("viewing product", "id", id)
		resp, err := client.Get(dbAddr + "/products/" + id)
		if err != nil {
			logger.Warn("db call failed", "error", err)
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		w.Write(buf.Bytes())
	})

	mux.HandleFunc("POST /cart/add", func(w http.ResponseWriter, r *http.Request) {
		var req CartAddRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
			return
		}
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = "default"
		}
		logger.Info("adding to cart", "session", sessionID, "product_id", req.ProductID, "quantity", req.Quantity)

		// Verify product exists via DB
		resp, err := client.Get(dbAddr + "/products/" + req.ProductID)
		if err != nil || resp.StatusCode != http.StatusOK {
			logger.Warn("product lookup failed", "product_id", req.ProductID)
			http.Error(w, `{"error":"product not found"}`, http.StatusNotFound)
			return
		}
		resp.Body.Close()

		mu.Lock()
		carts[sessionID] = append(carts[sessionID], CartItem{ProductID: req.ProductID, Quantity: req.Quantity})
		items := carts[sessionID]
		mu.Unlock()

		logger.Debug("cart updated", "session", sessionID, "item_count", len(items))
		writeJSON(w, map[string]any{"status": "added", "cart_size": len(items)})
	})

	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			sessionID = "default"
		}

		mu.Lock()
		items := carts[sessionID]
		delete(carts, sessionID)
		mu.Unlock()

		if len(items) == 0 {
			logger.Warn("checkout with empty cart", "session", sessionID)
			http.Error(w, `{"error":"cart is empty"}`, http.StatusBadRequest)
			return
		}

		// Simulate checkout processing time
		dur := time.Duration(50+rand.IntN(200)) * time.Millisecond
		time.Sleep(dur)
		if dur > 150*time.Millisecond {
			logger.Warn("slow checkout processing", "duration_ms", dur.Milliseconds(), "session", sessionID)
		}

		order := Order{
			ID:    fmt.Sprintf("ORD-%d", rand.IntN(100000)),
			Items: items,
			Total: float64(len(items)) * 29.99, // simplified
		}
		logger.Info("checkout complete", "order_id", order.ID, "items", len(items), "total", order.Total)
		writeJSON(w, order)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}

// --- API mode: public-facing gateway ---

func apiHandler(logger *slog.Logger, catalogAddr string) http.Handler {
	mux := http.NewServeMux()
	client := &http.Client{Timeout: 10 * time.Second}

	proxy := func(w http.ResponseWriter, r *http.Request, method, path string, body []byte) {
		var req *http.Request
		var err error
		if body != nil {
			req, err = http.NewRequestWithContext(r.Context(), method, catalogAddr+path, bytes.NewReader(body))
		} else {
			req, err = http.NewRequestWithContext(r.Context(), method, catalogAddr+path, nil)
		}
		if err != nil {
			logger.Warn("failed to create request", "error", err)
			http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if sid := r.Header.Get("X-Session-ID"); sid != "" {
			req.Header.Set("X-Session-ID", sid)
		}

		start := time.Now()
		resp, err := client.Do(req)
		elapsed := time.Since(start)
		if err != nil {
			logger.Warn("catalog call failed", "path", path, "error", err)
			http.Error(w, `{"error":"service unavailable"}`, http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		logger.Debug("catalog response", "path", path, "status", resp.StatusCode, "duration_ms", elapsed.Milliseconds())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		w.Write(buf.Bytes())
	}

	mux.HandleFunc("GET /products", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("GET /products")
		proxy(w, r, "GET", "/products", nil)
	})

	mux.HandleFunc("GET /products/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		logger.Info("GET /products/"+id, "id", id)
		proxy(w, r, "GET", "/products/"+id, nil)
	})

	mux.HandleFunc("POST /cart/add", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		buf.ReadFrom(r.Body)
		logger.Info("POST /cart/add")
		proxy(w, r, "POST", "/cart/add", buf.Bytes())
	})

	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		logger.Info("POST /checkout")
		proxy(w, r, "POST", "/checkout", nil)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}

// --- Instrumentation middleware ---

func instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" || r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		requestsInFlight.Inc()
		defer requestsInFlight.Dec()

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(sw, r)
		dur := time.Since(start)

		path := r.URL.Path
		requestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(sw.status)).Inc()
		requestDuration.WithLabelValues(r.Method, path).Observe(dur.Seconds())
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// --- Load generator ---

func runLoadGen(ctx context.Context, logger *slog.Logger, apiAddr string) {
	client := &http.Client{Timeout: 10 * time.Second}
	sessionCounter := 0

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sessionCounter++
		sessionID := fmt.Sprintf("session-%d", sessionCounter)

		// Browse products
		doReq(ctx, client, logger, "GET", apiAddr+"/products", nil, sessionID)
		sleep(ctx, 200*time.Millisecond)

		// View a random product
		productID := strconv.Itoa(1 + rand.IntN(5))
		doReq(ctx, client, logger, "GET", apiAddr+"/products/"+productID, nil, sessionID)
		sleep(ctx, 150*time.Millisecond)

		// Add to cart
		body, _ := json.Marshal(CartAddRequest{ProductID: productID, Quantity: 1 + rand.IntN(3)})
		doReq(ctx, client, logger, "POST", apiAddr+"/cart/add", body, sessionID)
		sleep(ctx, 300*time.Millisecond)

		// Sometimes add another item
		if rand.IntN(2) == 0 {
			productID2 := strconv.Itoa(1 + rand.IntN(5))
			body2, _ := json.Marshal(CartAddRequest{ProductID: productID2, Quantity: 1})
			doReq(ctx, client, logger, "POST", apiAddr+"/cart/add", body2, sessionID)
			sleep(ctx, 200*time.Millisecond)
		}

		// Checkout
		doReq(ctx, client, logger, "POST", apiAddr+"/checkout", nil, sessionID)
		sleep(ctx, 500*time.Millisecond)
	}
}

func doReq(ctx context.Context, client *http.Client, logger *slog.Logger, method, url string, body []byte, sessionID string) {
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		logger.Warn("loadgen: failed to create request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-ID", sessionID)

	start := time.Now()
	resp, err := client.Do(req)
	dur := time.Since(start)
	if err != nil {
		logger.Warn("loadgen: request failed", "method", method, "url", url, "error", err)
		return
	}
	resp.Body.Close()
	logger.Debug("loadgen: request complete", "method", method, "url", url, "status", resp.StatusCode, "duration_ms", dur.Milliseconds())
}

func sleep(ctx context.Context, d time.Duration) {
	jitter := time.Duration(rand.IntN(int(d / 2)))
	select {
	case <-ctx.Done():
	case <-time.After(d + jitter):
	}
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Main ---

func main() {
	mode := flag.String("mode", "api", "Run mode: api, catalog, db, loadgen")
	addr := flag.String("addr", ":8080", "Listen address")
	catalogAddr := flag.String("catalog-addr", "http://shop-catalog:8080", "Catalog service address")
	dbAddr := flag.String("db-addr", "http://shop-db:8080", "DB service address")
	apiAddr := flag.String("api-addr", "http://shop-api:8080", "API address (for loadgen)")
	logLevel := flag.String("log-level", "info", "Log level: debug, info, warn, error")
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	logger = logger.With("service", *mode)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *mode == "loadgen" {
		logger.Info("starting load generator", "target", *apiAddr)
		runLoadGen(ctx, logger, *apiAddr)
		return
	}

	var handler http.Handler
	switch *mode {
	case "api":
		handler = apiHandler(logger, *catalogAddr)
	case "catalog":
		handler = catalogHandler(logger, *dbAddr)
	case "db":
		handler = dbHandler(logger)
	default:
		logger.Error("unknown mode", "mode", *mode)
		os.Exit(1)
	}

	srv := &http.Server{Addr: *addr, Handler: instrument(handler)}

	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		srv.Shutdown(shutdownCtx)
	}()

	logger.Info("starting server", "mode", *mode, "addr", *addr, "log_level", *logLevel)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}

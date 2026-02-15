package main

import (
	"context"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

	"insolventbydesign/internal/model"
	"insolventbydesign/internal/storage"
)

// APIServer provides HTTP endpoints for censorship cost analysis.
type APIServer struct {
	store       *storage.PostgresStore
	rateLimiter *rate.Limiter
	metrics     *Metrics
}

// Metrics tracks API performance.
type Metrics struct {
	requestsTotal   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  prometheus.Gauge
}

func newMetrics() *Metrics {
	m := &Metrics{
		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_requests_total",
				Help: "Total number of API requests",
			},
			[]string{"endpoint", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "api_request_duration_seconds",
				Help:    "API request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"endpoint"},
		),
		activeRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "api_active_requests",
				Help: "Number of active API requests",
			},
		),
	}

	prometheus.MustRegister(m.requestsTotal, m.requestDuration, m.activeRequests)
	return m
}

func NewAPIServer(store *storage.PostgresStore) *APIServer {
	return &APIServer{
		store:       store,
		rateLimiter: rate.NewLimiter(rate.Limit(100), 200), // 100 RPS burst 200
		metrics:     newMetrics(),
	}
}

// CensorshipCostRequest represents the API request payload.
type CensorshipCostRequest struct {
	StartSlot          uint64  `json:"start_slot"`
	EndSlot            uint64  `json:"end_slot"`
	TopKBuilders       int     `json:"top_k_builders"`
	SuccessProbability float64 `json:"success_probability"`
	ETHPriceUSD        float64 `json:"eth_price_usd,omitempty"`
}

// CensorshipCostResponse represents the API response.
type CensorshipCostResponse struct {
	StartSlot            uint64        `json:"start_slot"`
	EndSlot              uint64        `json:"end_slot"`
	DurationSlots        uint64        `json:"duration_slots"`
	TotalCostETH         string        `json:"total_cost_eth"`
	TotalCostUSD         float64       `json:"total_cost_usd,omitempty"`
	BuilderConcentration float64       `json:"builder_concentration"`
	EffectiveCostETH     string        `json:"effective_cost_eth"`
	BreakevenTVLUSD      float64       `json:"breakeven_tvl_usd,omitempty"`
	TopBuilders          []BuilderInfo `json:"top_builders"`
}

type BuilderInfo struct {
	Pubkey     string  `json:"pubkey"`
	BlockCount uint64  `json:"block_count"`
	Percentage float64 `json:"percentage"`
}

// HealthResponse represents health check response.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
}

func (s *APIServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.rateLimiter.Allow() {
			s.metrics.requestsTotal.WithLabelValues(r.URL.Path, "429").Inc()
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		s.metrics.activeRequests.Inc()
		defer s.metrics.activeRequests.Dec()

		next.ServeHTTP(w, r)

		duration := time.Since(start).Seconds()
		s.metrics.requestDuration.WithLabelValues(r.URL.Path).Observe(duration)
	})
}

// HandleHealth returns API health status.
func (s *APIServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleComputeCensorshipCost computes censorship cost for a slot range.
func (s *APIServer) HandleComputeCensorshipCost(w http.ResponseWriter, r *http.Request) {
	var req CensorshipCostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validation
	if req.EndSlot <= req.StartSlot {
		http.Error(w, "end_slot must be greater than start_slot", http.StatusBadRequest)
		return
	}

	if req.TopKBuilders < 1 || req.TopKBuilders > 100 {
		http.Error(w, "top_k_builders must be between 1 and 100", http.StatusBadRequest)
		return
	}

	if req.SuccessProbability <= 0 || req.SuccessProbability > 1 {
		http.Error(w, "success_probability must be between 0 and 1", http.StatusBadRequest)
		return
	}

	// Fetch data from database
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	bribes, err := s.store.GetSlotRange(ctx, req.StartSlot, req.EndSlot)
	if err != nil {
		log.Printf("Failed to fetch bribes: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(bribes) == 0 {
		http.Error(w, "No data found for specified slot range", http.StatusNotFound)
		return
	}

	// Compute censorship cost
	tau := req.EndSlot - req.StartSlot + 1
	totalCost, err := model.CensorshipCost(bribes, tau)
	if err != nil {
		log.Printf("Failed to compute cost: %v", err)
		http.Error(w, "Failed to compute censorship cost", http.StatusInternalServerError)
		return
	}

	// Compute builder concentration
	alpha, builderStats, err := model.ComputeBuilderConcentration(bribes, req.TopKBuilders)
	if err != nil {
		log.Printf("Failed to compute concentration: %v", err)
		http.Error(w, "Failed to compute builder concentration", http.StatusInternalServerError)
		return
	}

	// Compute effective cost
	effectiveCost := new(big.Float).Mul(
		new(big.Float).SetInt(totalCost),
		big.NewFloat(1.0-alpha),
	)

	// Convert to ETH
	weiPerEth := new(big.Float).SetInt(big.NewInt(1e18))
	totalCostETH := new(big.Float).Quo(new(big.Float).SetInt(totalCost), weiPerEth)
	effectiveCostETH := new(big.Float).Quo(effectiveCost, weiPerEth)

	// Build response
	response := CensorshipCostResponse{
		StartSlot:            req.StartSlot,
		EndSlot:              req.EndSlot,
		DurationSlots:        tau,
		TotalCostETH:         totalCostETH.Text('f', 6),
		BuilderConcentration: alpha,
		EffectiveCostETH:     effectiveCostETH.Text('f', 6),
		TopBuilders:          make([]BuilderInfo, 0),
	}

	// Compute USD values if ETH price provided
	if req.ETHPriceUSD > 0 {
		totalCostETHFloat, _ := totalCostETH.Float64()
		effectiveCostETHFloat, _ := effectiveCostETH.Float64()

		response.TotalCostUSD = totalCostETHFloat * req.ETHPriceUSD
		response.BreakevenTVLUSD = (effectiveCostETHFloat * req.ETHPriceUSD) / req.SuccessProbability
	}

	// Add top builders
	totalBlocks := uint64(len(bribes))
	for i := 0; i < req.TopKBuilders && i < len(builderStats); i++ {
		response.TopBuilders = append(response.TopBuilders, BuilderInfo{
			Pubkey:     builderStats[i].BuilderPubkey,
			BlockCount: builderStats[i].BlockCount,
			Percentage: float64(builderStats[i].BlockCount) / float64(totalBlocks) * 100,
		})
	}

	s.metrics.requestsTotal.WithLabelValues("/api/v1/censorship-cost", "200").Inc()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetBuilderStats returns builder statistics.
func (s *APIServer) HandleGetBuilderStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	stats, err := s.store.GetBuilderStats(ctx)
	if err != nil {
		log.Printf("Failed to fetch builder stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func main() {
	// Database configuration from environment
	dbConfig := storage.Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		User:     getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		Database: getEnv("DB_NAME", "censorship_db"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	store, err := storage.NewPostgresStore(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer store.Close()

	server := NewAPIServer(store)

	// Setup router
	r := mux.NewRouter()
	r.Use(server.rateLimitMiddleware)
	r.Use(server.metricsMiddleware)

	// API endpoints
	r.HandleFunc("/health", server.HandleHealth).Methods("GET")
	r.HandleFunc("/api/v1/censorship-cost", server.HandleComputeCensorshipCost).Methods("POST")
	r.HandleFunc("/api/v1/builders", server.HandleGetBuilderStats).Methods("GET")

	// Prometheus metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// HTTP server
	port := getEnv("PORT", "8080")
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		log.Printf("API server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

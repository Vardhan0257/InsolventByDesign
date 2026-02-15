#!/bin/bash

# Run comprehensive benchmarks for InsolventByDesign

set -e

echo "======================================="
echo "InsolventByDesign Performance Benchmarks"
echo "======================================="
echo ""

# Go benchmarks
echo "[1/3] Running Go benchmarks..."
echo "----------------------------------------"
go test -bench=. -benchmem -benchtime=10s ./internal/model | tee analysis/reports/benchmark_go.txt
echo ""

# API load testing (requires running API server)
echo "[2/3] API Load Testing..."
echo "----------------------------------------"
echo "Starting API server..."
go run cmd/api-server/main.go &
API_PID=$!
sleep 3

# Simple load test
echo "Running concurrent requests..."
for i in {1..1000}; do
  curl -s -X GET http://localhost:8080/health > /dev/null &
done
wait

echo "Load test complete (1000 concurrent health checks)"
kill $API_PID 2>/dev/null || true
echo ""

# Database benchmarks
echo "[3/3] Database Performance..."
echo "----------------------------------------"
echo "Benchmark: Batch insert performance"
go test -bench=BenchmarkBatchInsert -benchtime=5s ./internal/storage 2>/dev/null || echo "Database tests require running PostgreSQL"
echo ""

echo "======================================="
echo "Benchmarks Complete!"
echo "======================================="
echo ""
echo "Results saved to: analysis/reports/benchmark_go.txt"
echo ""
echo "Key Metrics:"
go test -bench=. -benchmem ./internal/model 2>/dev/null | grep "Benchmark" | head -5

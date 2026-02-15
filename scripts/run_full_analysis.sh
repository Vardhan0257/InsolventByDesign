#!/bin/bash

# Full analysis pipeline for InsolventByDesign
# This script fetches data, runs analysis, and generates reports

set -e

echo "====================================="
echo "InsolventByDesign Analysis Pipeline"
echo "====================================="
echo ""

# Configuration
DATA_DIR="data"
ANALYSIS_DIR="analysis"
START_SLOT=${START_SLOT:-8000000}
END_SLOT=${END_SLOT:-8100000}
ETH_PRICE=${ETH_PRICE:-3500}
BRIDGE_TVL=${BRIDGE_TVL:-500000000}

# Create directories
mkdir -p $DATA_DIR
mkdir -p $ANALYSIS_DIR/plots
mkdir -p $ANALYSIS_DIR/reports

# Step 1: Build binaries
echo "[1/6] Building binaries..."
go build -o bin/fetch-relay ./cmd/fetch-relay
go build -o bin/threshold-analysis ./cmd/threshold-analysis
go build -o bin/analysis ./cmd/analysis
go build -o bin/api-server ./cmd/api-server
echo "✓ Build complete"
echo ""

# Step 2: Fetch relay data
echo "[2/6] Fetching relay data (slots $START_SLOT to $END_SLOT)..."
# Note: Implement actual fetching in fetch-relay
# For now, generate sample data
go run cmd/fetch-relay/main.go --start=$START_SLOT --end=$END_SLOT --output=$DATA_DIR/bribes.json
echo "✓ Data fetched"
echo ""

# Step 3: Statistical summary
echo "[3/6] Computing statistical summary..."
./bin/analysis --data=$DATA_DIR/bribes.json --mode=summary > $ANALYSIS_DIR/reports/summary.txt
echo "✓ Summary complete"
echo ""

# Step 4: Rolling statistics
echo "[4/6] Computing rolling statistics..."
./bin/analysis --data=$DATA_DIR/bribes.json --mode=rolling --window=1000 > $ANALYSIS_DIR/reports/rolling.txt
echo "✓ Rolling analysis complete"
echo ""

# Step 5: Concentration analysis
echo "[5/6] Analyzing builder concentration..."
./bin/analysis --data=$DATA_DIR/bribes.json --mode=concentration --window=1000 > $ANALYSIS_DIR/reports/concentration.txt
echo "✓ Concentration analysis complete"
echo ""

# Step 6: Monte Carlo simulation
echo "[6/6] Running Monte Carlo simulation..."
./bin/analysis --data=$DATA_DIR/bribes.json \
    --mode=montecarlo \
    --tau=1800 \
    --eth-price=$ETH_PRICE \
    --bridge-tvl=$BRIDGE_TVL \
    --success-prob=0.8 \
    --simulations=100000 > $ANALYSIS_DIR/reports/monte_carlo.txt
echo "✓ Monte Carlo complete"
echo ""

# Summary
echo "====================================="
echo "Analysis Complete!"
echo "====================================="
echo "Reports generated in: $ANALYSIS_DIR/reports/"
echo ""
echo "View results:"
echo "  cat $ANALYSIS_DIR/reports/summary.txt"
echo "  cat $ANALYSIS_DIR/reports/monte_carlo.txt"
echo ""
echo "Next steps:"
echo "  1. Review analysis reports"
echo "  2. Start API server: ./bin/api-server"
echo "  3. Deploy to production: docker-compose up -d"

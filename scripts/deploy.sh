#!/bin/bash

# Production deployment script

set -e

echo "======================================="
echo "InsolventByDesign Deployment"
echo "======================================="
echo ""

DEPLOYMENT_MODE=${1:-docker}

if [ "$DEPLOYMENT_MODE" = "docker" ]; then
    echo "Deploying with Docker Compose..."
    echo "----------------------------------------"
    
    # Build images
    echo "Building Docker images..."
    docker-compose build
    
    # Start services
    echo "Starting services..."
    docker-compose up -d
    
    # Wait for health checks
    echo "Waiting for services to be healthy..."
    sleep 10
    
    # Check status
    echo ""
    echo "Service Status:"
    docker-compose ps
    
    echo ""
    echo "✓ Deployment complete!"
    echo ""
    echo "Access points:"
    echo "  API:        http://localhost:8080"
    echo "  Prometheus: http://localhost:9090"
    echo "  Grafana:    http://localhost:3000 (admin/admin)"
    echo ""
    echo "View logs: docker-compose logs -f"
    
elif [ "$DEPLOYMENT_MODE" = "kubernetes" ]; then
    echo "Deploying to Kubernetes..."
    echo "----------------------------------------"
    
    # Apply manifests
    echo "Applying Kubernetes manifests..."
    kubectl apply -f k8s/deployment.yaml
    
    # Wait for rollout
    echo "Waiting for deployment rollout..."
    kubectl rollout status deployment/api-server -n censorship-analysis
    
    # Get service info
    echo ""
    echo "✓ Deployment complete!"
    echo ""
    kubectl get pods -n censorship-analysis
    kubectl get services -n censorship-analysis
    
    echo ""
    echo "Get external IP: kubectl get service api-service -n censorship-analysis"
    
else
    echo "Usage: ./deploy.sh [docker|kubernetes]"
    exit 1
fi

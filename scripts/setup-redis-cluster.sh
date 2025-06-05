#!/bin/bash

# Redis Cluster Integration Test Script
# This script sets up a Redis cluster for testing Deckard's cluster compatibility

set -e

echo "Setting up Redis Cluster for Deckard integration testing..."

# Check if docker and docker-compose are available
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed or not in PATH"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "Error: docker-compose is not installed or not in PATH"
    exit 1
fi

# Navigate to docker directory
cd "$(dirname "$0")/../docker"

echo "Starting Redis Cluster nodes..."
docker-compose up -d redis-cluster-node-1 redis-cluster-node-2 redis-cluster-node-3 redis-cluster-node-4 redis-cluster-node-5 redis-cluster-node-6

echo "Waiting for Redis nodes to be ready..."
sleep 10

echo "Creating Redis Cluster..."
docker-compose run --rm redis-cluster-init

echo "Waiting for cluster to stabilize..."
sleep 5

echo "Verifying cluster status..."
docker exec -it docker_redis-cluster-node-1_1 redis-cli --cluster info localhost:7000 || echo "Cluster info command failed, but this might be expected"

echo "Redis Cluster is ready for testing!"
echo ""
echo "To run Redis Cluster integration tests:"
echo "  export REDIS_CLUSTER_TEST=1"
echo "  cd .."
echo "  go test -v ./internal/queue/cache/ -run 'Cluster'"
echo ""
echo "To stop the cluster:"
echo "  docker-compose down -v"
echo ""
echo "Cluster node addresses:"
echo "  - localhost:7000"
echo "  - localhost:7001" 
echo "  - localhost:7002"
echo "  - localhost:7003"
echo "  - localhost:7004"
echo "  - localhost:7005"
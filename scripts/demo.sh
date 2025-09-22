#!/bin/bash

set -e

echo "🚀 Tezos Delegation Service Demo"
echo "================================="
echo ""
echo "This demo script shows how to test the Tezos Delegation Service API."
echo ""

BASE_URL="${1:-http://localhost:8080}"

echo "Using API URL: $BASE_URL"
echo ""

# Check if service is running
echo "1. Checking service health..."
echo "   GET $BASE_URL/health"
curl -s "$BASE_URL/health" | jq . || echo "Service not running. Please start with 'docker-compose up' or 'go run cmd/server/main.go'"
echo ""

echo "2. Getting all delegations..."
echo "   GET $BASE_URL/xtz/delegations"
curl -s "$BASE_URL/xtz/delegations" | jq . | head -50 || echo "Failed to get delegations"
echo ""

echo "3. Getting delegations for year 2022..."
echo "   GET $BASE_URL/xtz/delegations?year=2022"
curl -s "$BASE_URL/xtz/delegations?year=2022" | jq . | head -50 || echo "Failed to get delegations for 2022"
echo ""

echo "4. Getting service statistics..."
echo "   GET $BASE_URL/stats"
curl -s "$BASE_URL/stats" | jq . || echo "Failed to get stats"
echo ""

echo "5. Checking readiness..."
echo "   GET $BASE_URL/ready"
curl -s "$BASE_URL/ready" | jq . || echo "Failed to check readiness"
echo ""

echo "6. Getting Prometheus metrics..."
echo "   GET $BASE_URL/metrics"
curl -s "$BASE_URL/metrics" | head -20 || echo "Failed to get metrics"
echo ""

echo "✅ Demo complete!"
echo ""
echo "To run the full service stack:"
echo "  docker-compose up -d"
echo ""
echo "To view logs:"
echo "  docker-compose logs -f tezos-delegation-service"
echo ""
echo "To stop services:"
echo "  docker-compose down"
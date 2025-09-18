#!/bin/bash

set -e

echo "🧪 Running Tezos Delegation Service Tests"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Run Go tests
echo -e "\n${YELLOW}Running Unit Tests...${NC}"
go test -v -race -cover ./... || {
    echo -e "${RED}Unit tests failed!${NC}"
    exit 1
}

echo -e "\n${GREEN}✅ All tests passed!${NC}"

# Build binary
echo -e "\n${YELLOW}Building binary...${NC}"
go build -o bin/tezos-delegation-service cmd/server/main.go || {
    echo -e "${RED}Build failed!${NC}"
    exit 1
}

echo -e "${GREEN}✅ Build successful!${NC}"

# Check formatting
echo -e "\n${YELLOW}Checking code formatting...${NC}"
if [ -n "$(gofmt -l .)" ]; then
    echo -e "${RED}Code is not formatted. Run 'go fmt ./...'${NC}"
    gofmt -l .
else
    echo -e "${GREEN}✅ Code is properly formatted!${NC}"
fi

echo -e "\n${GREEN}🎉 All checks passed successfully!${NC}"
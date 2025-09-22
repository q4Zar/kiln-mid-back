.PHONY: help build run test clean docker-build docker-up docker-down migrate lint fmt

# Variables
APP_NAME = tezos-delegation-service
DOCKER_IMAGE = $(APP_NAME):latest
GO = go
GOFLAGS = -v
MAIN_PATH = cmd/server/main.go
BIN_PATH = bin/$(APP_NAME)

# Colors for output
GREEN = \033[0;32m
YELLOW = \033[0;33m
NC = \033[0m # No Color

## help: Display this help message
help:
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^##' Makefile | sed 's/## /  /'

## build: Build the application binary
build:
	@echo "$(YELLOW)Building $(APP_NAME)...$(NC)"
	@$(GO) build $(GOFLAGS) -o $(BIN_PATH) $(MAIN_PATH)
	@echo "$(GREEN)Build complete: $(BIN_PATH)$(NC)"

## run: Run the application locally
run:
	@echo "$(YELLOW)Running $(APP_NAME)...$(NC)"
	@$(GO) run $(MAIN_PATH)

## test: Run all unit tests
test:
	@echo "$(YELLOW)Running unit tests...$(NC)"
	@$(GO) test -v -short -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)Tests complete$(NC)"

## test-unit: Run unit tests with verbose output
test-unit:
	@echo "$(YELLOW)Running unit tests...$(NC)"
	@$(GO) test -v -short -race ./...
	@echo "$(GREEN)Unit tests complete$(NC)"

## test-integration: Run integration tests
test-integration:
	@echo "$(YELLOW)Running integration tests...$(NC)"
	@$(GO) test -v -race -tags=integration ./internal/...
	@echo "$(GREEN)Integration tests complete$(NC)"

## test-all: Run all tests (unit and integration)
test-all:
	@echo "$(YELLOW)Running all tests...$(NC)"
	@$(GO) test -v -race -coverprofile=coverage.out ./...
	@echo "$(GREEN)All tests complete$(NC)"

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "$(YELLOW)Generating coverage report...$(NC)"
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@$(GO) tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

## test-coverage-detailed: Generate detailed coverage report with function breakdown
test-coverage-detailed:
	@echo "$(YELLOW)Running tests with detailed coverage...$(NC)"
	@$(GO) test -v -race -coverprofile=coverage.out -coverpkg=./... ./...
	@echo "$(YELLOW)Generating detailed coverage report...$(NC)"
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@$(GO) tool cover -func=coverage.out
	@echo "$(GREEN)Detailed coverage report generated: coverage.html$(NC)"

## test-benchmark: Run benchmark tests
test-benchmark:
	@echo "$(YELLOW)Running benchmark tests...$(NC)"
	@$(GO) test -bench=. -benchmem ./...
	@echo "$(GREEN)Benchmark tests complete$(NC)"

## test-specific: Run tests for a specific package (usage: make test-specific PKG=./internal/application)
test-specific:
	@if [ -z "$(PKG)" ]; then \
		echo "$(YELLOW)Please specify a package: make test-specific PKG=./internal/application$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Running tests for $(PKG)...$(NC)"
	@$(GO) test -v -race $(PKG)
	@echo "$(GREEN)Tests complete for $(PKG)$(NC)"

## test-watch: Run tests in watch mode (requires entr)
test-watch:
	@echo "$(YELLOW)Running tests in watch mode...$(NC)"
	@command -v entr >/dev/null 2>&1 || { echo "$(YELLOW)Installing entr...$(NC)"; brew install entr; }
	@find . -name '*.go' | entr -c $(GO) test -v -short ./...

## lint: Run linters
lint:
	@echo "$(YELLOW)Running linters...$(NC)"
	@golangci-lint run --timeout 5m
	@echo "$(GREEN)Linting complete$(NC)"

## fmt: Format code
fmt:
	@echo "$(YELLOW)Formatting code...$(NC)"
	@$(GO) fmt ./...
	@echo "$(GREEN)Formatting complete$(NC)"

## clean: Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(BIN_PATH) coverage.out coverage.html
	@echo "$(GREEN)Clean complete$(NC)"

## docker-build: Build Docker image
docker-build:
	@echo "$(YELLOW)Building Docker image...$(NC)"
	@docker build -t $(DOCKER_IMAGE) .
	@echo "$(GREEN)Docker build complete$(NC)"

## docker-up: Start all services with docker-compose
docker-up:
	@echo "$(YELLOW)Starting services...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)Services started$(NC)"
	@echo "API: http://localhost:8080"
	@echo "Metrics: http://localhost:9090"
	@echo "Grafana: http://localhost:3000 (admin/admin)"

## docker-down: Stop all services
docker-down:
	@echo "$(YELLOW)Stopping services...$(NC)"
	@docker-compose down
	@echo "$(GREEN)Services stopped$(NC)"

## docker-logs: View service logs
docker-logs:
	@docker-compose logs -f tezos-delegation-service

## docker-clean: Stop services and remove volumes
docker-clean:
	@echo "$(YELLOW)Cleaning Docker environment...$(NC)"
	@docker-compose down -v
	@echo "$(GREEN)Docker environment cleaned$(NC)"

## backup: Create database backup
backup:
	@echo "$(YELLOW)Creating database backup...$(NC)"
	@mkdir -p backups
	@docker-compose exec postgres sh -c 'PGPASSWORD=tezos pg_dump -U tezos -d tezos_delegations --no-owner --no-privileges --data-only --table=delegations --table=schema_migrations --column-inserts --on-conflict-do-nothing' | gzip > backups/tezos_delegations_$$(date +%Y%m%d_%H%M%S).sql.gz
	@ln -sf tezos_delegations_$$(date +%Y%m%d_%H%M%S).sql.gz backups/latest.sql.gz
	@echo "$(GREEN)Backup created: backups/latest.sql.gz$(NC)"
	@ls -lh backups/*.sql.gz | tail -1

## restore: Restore database from latest backup
restore:
	@if [ ! -f backups/latest.sql.gz ]; then \
		echo "$(YELLOW)No backup found at backups/latest.sql.gz$(NC)"; \
		echo "Available backups:"; \
		ls -lh backups/*.sql.gz 2>/dev/null || echo "No backups found"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Restoring from latest backup...$(NC)"
	@zcat backups/latest.sql.gz | docker-compose exec -T postgres psql -U tezos -d tezos_delegations
	@echo "$(GREEN)Restore completed!$(NC)"

## db-status: Show database status
db-status:
	@echo "$(YELLOW)Database Status:$(NC)"
	@docker-compose exec postgres psql -U tezos -d tezos_delegations -t -c "SELECT 'Delegations: ' || COUNT(*), 'Last Level: ' || MAX(level), 'Date Range: ' || MIN(timestamp)::date || ' to ' || MAX(timestamp)::date FROM delegations;" | sed 's/|//g'

## deps: Download dependencies
deps:
	@echo "$(YELLOW)Downloading dependencies...$(NC)"
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "$(GREEN)Dependencies downloaded$(NC)"

## install-tools: Install development tools
install-tools:
	@echo "$(YELLOW)Installing development tools...$(NC)"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)Tools installed$(NC)"

.DEFAULT_GOAL := help 
# Tezos Delegation Service

A high-performance Go service that continuously indexes Tezos blockchain delegations and exposes them through a REST API with advanced filtering capabilities.

## Features

- **Real-time Delegation Indexing**: Continuously polls and indexes new delegations from the Tezos blockchain
- **Historical Data Support**: Automatically indexes historical delegation data from configurable start date
- **REST API**: Exposes delegations through a clean REST API with year-based filtering
- **Production-Ready**: Includes health checks, metrics, monitoring, and graceful shutdown
- **Clean Architecture**: Follows domain-driven design principles with clear separation of concerns
- **Comprehensive Testing**: Unit tests, integration tests, and CI/CD pipeline
- **Containerized**: Docker and docker-compose support for easy deployment
- **Monitoring**: Prometheus metrics and Grafana dashboards included

## Quick Start

### Prerequisites

- Go 1.22+
- Docker and Docker Compose
- PostgreSQL 16+ (or use Docker)

### Running with Docker Compose

1. Clone the repository:
```bash
git clone https://github.com/nacon-liveops/tezos-delegation-service.git
cd tezos-delegation-service
```

2. Start all services:
```bash
docker-compose up -d
```

This will start:
- PostgreSQL database
- Tezos Delegation Service
- Prometheus (metrics collection)
- Grafana (visualization)

3. Access the services:
- API: http://localhost:8080
- Metrics: http://localhost:9090
- Grafana: http://localhost:3000 (admin/admin)

### Running Locally

1. Install dependencies:
```bash
go mod download
```

2. Set up PostgreSQL and run migrations:
```bash
psql -U postgres -c "CREATE DATABASE tezos_delegations;"
psql -U postgres -d tezos_delegations -f migrations/001_create_delegations_table.sql
```

3. Configure environment:
```bash
cp .env.example .env
# Edit .env with your configuration
```

4. Run the service:
```bash
go run cmd/server/main.go
```

## API Documentation

### Get Delegations

Retrieve all delegations, optionally filtered by year.

**Endpoint:** `GET /xtz/delegations`

**Query Parameters:**
- `year` (optional): Filter delegations by year (YYYY format)

**Response:**
```json
{
  "data": [
    {
      "timestamp": "2022-05-05T06:29:14Z",
      "amount": "125896",
      "delegator": "tz1a1SAaXRt9yoGMx29rh9FsBF4UzmvojdTL",
      "level": "2338084"
    }
  ]
}
```

**Example Requests:**
```bash
# Get all delegations
curl http://localhost:8080/xtz/delegations

# Get delegations from 2022
curl http://localhost:8080/xtz/delegations?year=2022
```

### Health Check

**Endpoint:** `GET /health`

Returns service health status and basic statistics.

### Metrics

**Endpoint:** `GET /metrics`

Prometheus-formatted metrics for monitoring.

## Configuration

Configuration can be set via environment variables or `.env` file:

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://tezos:tezos@localhost:5432/tezos_delegations` |
| `SERVER_PORT` | API server port | `8080` |
| `TZKT_API_URL` | TzKT API base URL | `https://api.tzkt.io` |
| `POLLING_INTERVAL` | How often to poll for new delegations | `30s` |
| `HISTORICAL_INDEXING` | Enable historical data indexing | `true` |
| `HISTORICAL_START_DATE` | Start date for historical indexing | `2021-01-01` |
| `LOG_LEVEL` | Logging level (debug/info/warn/error) | `info` |
| `METRICS_PORT` | Prometheus metrics port | `9090` |

## Development

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Building

```bash
# Build binary
go build -o tezos-delegation-service cmd/server/main.go

# Build Docker image
docker build -t tezos-delegation-service:latest .
```

## Monitoring

Access Grafana at http://localhost:3000 to view:
- Delegations processed rate
- API request latency
- Error rates
- Database connection metrics
- Historical indexing progress

## License

MIT License

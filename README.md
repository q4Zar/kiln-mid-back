# Tezos Delegation Service

[![CI Pipeline](https://github.com/q4ZAr/kiln-mid-back/actions/workflows/ci.yml/badge.svg)](https://github.com/q4ZAr/kiln-mid-back/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/q4ZAr/kiln-mid-back/tezos-delegation-service)](https://goreportcard.com/report/github.com/q4ZAr/kiln-mid-back/tezos-delegation-service)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A production-ready Go service that continuously indexes Tezos blockchain delegations from the TzKT API and exposes them through a RESTful API with advanced filtering capabilities.

## 🚀 Quick Start with Docker Compose

The service is designed to run with Docker Compose, which orchestrates the entire startup process:

1. **Database initialization** - Sets up PostgreSQL with required schemas
2. **Backup restoration** - Loads existing data if available
3. **Test execution** - Runs unit tests to verify integrity
4. **Service startup** - Launches the main application

### Starting the Service

```bash
# Clone the repository
git clone https://github.com/q4ZAr/kiln-mid-back.git
cd kiln-mid-back

# Start all services
docker-compose up
```

The startup process will:
- ✓ Wait for PostgreSQL to be ready
- ✓ Run database migrations
- ✓ Restore from backup (if `backups/latest.sql.gz` exists)
- ✓ Execute unit tests
- ✓ Start the Tezos Delegation Service

If any step fails, the service will not start, ensuring data integrity.

### Service Endpoints

Once running, access the services at:
- **API**: http://localhost:8080
- **Health Check**: http://localhost:8080/health
- **Metrics**: http://localhost:9090/metrics
- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9091

## 📋 Features

- **Real-time Indexing**: Continuously polls and indexes new delegations from the Tezos blockchain
- **Historical Data Support**: Automatically indexes historical delegation data
- **RESTful API**: Clean API with year-based filtering
- **High Performance**: Batch processing and optimized database queries
- **Production Ready**: Health checks, metrics, and graceful shutdown
- **Observability**: Built-in Prometheus metrics and Grafana dashboards
- **Automated Testing**: Tests run automatically before service startup
- **Backup & Restore**: Automatic backup creation and restoration

## 🧪 Testing

### Automated Testing in Docker

Tests run automatically when you start the service with `docker-compose up`. Control this behavior with environment variables:

```yaml
# docker-compose.yml
environment:
  RUN_TESTS: "true"              # Run unit tests on startup
  RUN_INTEGRATION_TESTS: "false" # Run integration tests
  SKIP_TEST_FAILURE: "false"     # Continue even if tests fail
```

### Manual Testing

Run tests locally with Make commands:

```bash
make test                 # Run unit tests
make test-integration     # Run integration tests  
make test-all            # Run all tests
make test-coverage       # Generate coverage report
make test-benchmark      # Run benchmarks
make test-watch          # Watch mode for TDD
```

### Test Structure

```
internal/
├── application/
│   └── service_test.go        # Service layer tests
├── domain/
│   └── delegation_test.go     # Domain model tests
├── infrastructure/
│   ├── postgres/
│   │   └── repository_test.go # Database tests
│   └── tzkt/
│       └── client_test.go     # API client tests
├── interfaces/
│   └── http/
│       └── handlers_test.go   # HTTP handler tests
└── integration_test.go        # Integration test suite
```

### Coverage Goals

- Minimum: 60%
- Target: 80%
- Critical paths: 90%

## 📖 API Documentation

### Get Delegations

Retrieve delegations with optional filtering.

**Endpoint:** `GET /xtz/delegations`

**Query Parameters:**
- `year` (optional): Filter by year (2018-2100)

**Response:**
```json
{
  "data": [
    {
      "timestamp": "2022-05-05T06:29:14Z",
      "amount": "125896",
      "delegator": "tz1VSUr8wwNhLAzempoch5d6hLRiTh8Cjcjb",
      "level": "2338084"
    }
  ]
}
```

### Health Check

**Endpoint:** `GET /health`

Returns service health status and basic statistics.

### Statistics

**Endpoint:** `GET /stats`

Returns comprehensive statistics about indexed delegations.

## 🛠️ Development Setup

### Prerequisites

- Go 1.23+
- Docker 20.10+ and Docker Compose 2.0+
- PostgreSQL 16+ (optional, can use Docker)

### Local Development

1. **Install dependencies:**
```bash
go mod download
go mod tidy
```

2. **Set up PostgreSQL:**
```bash
# Using Docker
docker-compose up postgres

# Or manually
psql -U postgres -c "CREATE DATABASE tezos_delegations;"
```

3. **Run migrations:**
```bash
make migrate
```

4. **Configure environment:**
```bash
cp .env.example .env
# Edit .env with your settings
```

5. **Run the service:**
```bash
go run cmd/server/main.go
```

## 🔧 Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Required |
| `SERVER_PORT` | API server port | `8080` |
| `TZKT_API_URL` | TzKT API endpoint | `https://api.tzkt.io` |
| `POLLING_INTERVAL` | New data polling interval | `30s` |
| `HISTORICAL_INDEXING` | Enable historical data indexing | `true` |
| `HISTORICAL_START_DATE` | Start date for historical indexing | `2021-01-01` |
| `LOG_LEVEL` | Logging level | `info` |
| `RUN_TESTS` | Run tests on Docker startup | `true` |
| `RESTORE_BACKUP` | Restore from backup on startup | `true` |

### Docker Compose Settings

Modify `docker-compose.yml` to customize:
- Port mappings
- Volume mounts
- Environment variables
- Resource limits

## 📊 Monitoring

### Metrics

The service exposes Prometheus metrics at `/metrics`:
- `delegations_indexed_total` - Total delegations indexed
- `api_requests_total` - API request count
- `api_request_duration_seconds` - Request latency
- `indexing_errors_total` - Indexing error count

### Grafana Dashboards

Pre-configured dashboards available at http://localhost:3000:
- Service Overview
- API Performance
- Database Metrics
- Indexing Status

## 🔄 Backup & Restore

### Creating Backups

```bash
make backup
# Creates backups/tezos_delegations_YYYYMMDD_HHMMSS.sql.gz
```

### Restoring from Backup

Automatic restoration on startup if `backups/latest.sql.gz` exists:
```bash
# Place backup file
cp your-backup.sql.gz backups/latest.sql.gz

# Start services (will auto-restore)
docker-compose up
```

Manual restoration:
```bash
make restore
```

## 🏗️ Project Structure

```
.
├── cmd/server/          # Application entrypoint
├── internal/
│   ├── application/     # Business logic
│   ├── domain/          # Domain models
│   ├── infrastructure/  # External services
│   └── interfaces/      # API handlers
├── pkg/
│   ├── config/          # Configuration
│   ├── logger/          # Logging
│   └── metrics/         # Metrics collection
├── migrations/          # Database migrations
├── monitoring/          # Prometheus & Grafana configs
├── scripts/             # Utility scripts
├── backups/            # Database backups
└── docker-compose.yml  # Service orchestration
```

## 🚢 Deployment

### Production Deployment

1. **Configure environment:**
```bash
cp .env.example .env.production
# Set production values
```

2. **Build and deploy:**
```bash
docker-compose -f docker-compose.yml \
               -f docker-compose.prod.yml \
               up -d
```

3. **Verify deployment:**
```bash
curl https://your-domain.com/health
```

### Kubernetes Deployment

Helm charts available in `k8s/`:
```bash
helm install tezos-delegation ./k8s/helm
```

## 🐛 Troubleshooting

### Service Won't Start

1. Check test results in Docker logs:
```bash
docker-compose logs tezos-delegation-service
```

2. Verify database connection:
```bash
docker-compose exec postgres pg_isready
```

3. Check migrations:
```bash
docker-compose exec tezos-delegation-service migrate -path=/app/migrations -database=$DATABASE_URL up
```

### Tests Failing

1. Run tests locally:
```bash
make test-specific PKG=./internal/failing-package
```

2. Skip tests temporarily:
```yaml
environment:
  SKIP_TEST_FAILURE: "true"
```

### Performance Issues

1. Check metrics:
```bash
curl http://localhost:9090/metrics | grep -E "(latency|error)"
```

2. Review database queries:
```sql
SELECT * FROM pg_stat_statements ORDER BY total_time DESC LIMIT 10;
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

### Development Workflow

```bash
# Create branch
git checkout -b feature/your-feature

# Make changes and test
make test-watch

# Run full test suite
make test-all

# Check coverage
make test-coverage

# Commit changes
git add .
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/your-feature
```

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- TzKT API for blockchain data
- Tezos community for support
- Contributors and maintainers

## 📞 Support

For issues and questions:
- Create an [issue](https://github.com/q4ZAr/kiln-mid-back/issues)
- Check [documentation](./docs)
- Review [test examples](./internal/testutil)
# Tezos Delegation Service - Technical Interview Preparation

## 📋 Interview Structure
- **10 min**: Introductions
- **35 min**: Code Review based on the case study
- **45 min**: System Design exercise using Excalidraw

---

## 🏗️ Project Overview

### What It Does
The Tezos Delegation Service is a **production-ready Go microservice** that:
- **Continuously indexes** Tezos blockchain delegation operations from TzKT API
- **Stores** delegation data in PostgreSQL with deduplication
- **Exposes** RESTful API for querying delegations with year-based filtering
- **Monitors** health, metrics, and performance via Prometheus/Grafana

### Key Achievements from Logs
- Successfully indexed **522,387 delegations** (519,115 existing + 3,272 new)
- Achieves **batch processing** of 1,000 delegations per operation
- Implements **automatic test execution** before startup
- Provides **real-time monitoring** with health checks every 30 seconds
- Handles **concurrent requests** with proper request logging

---

## 🔍 Code Review Discussion Points

### 1. Architecture & Design Patterns

#### **Clean Architecture Implementation**
```
internal/
├── application/     # Business logic (Service layer)
├── domain/          # Core entities & interfaces
├── infrastructure/  # External dependencies (DB, API)
└── interfaces/      # API handlers & middleware
```
- **Strengths**: Clear separation of concerns, dependency injection, testable design
- **Discussion**: How would you extend this for multiple blockchain networks?

**Answer**: I would extend the architecture using a **Strategy Pattern** with blockchain-agnostic abstractions:

1. **Domain Layer Changes**:
   ```go
   // Create blockchain-agnostic interface
   type BlockchainDelegation interface {
       GetChain() string
       GetTimestamp() time.Time
       GetAmount() string
       GetDelegator() string
       GetLevel() string
       ToCommonFormat() Delegation
   }

   // Extend repository for multi-chain
   type DelegationRepository interface {
       SaveBatch(chain string, delegations []Delegation) error
       FindAll(chain string, year *int) ([]Delegation, error)
       GetLastIndexedLevel(chain string) (int64, error)
   }
   ```

2. **Infrastructure Layer - Adapter Pattern**:
   ```
   infrastructure/
   ├── blockchain/
   │   ├── tezos/
   │   │   ├── tzkt_client.go
   │   │   └── models.go
   │   ├── ethereum/
   │   │   ├── etherscan_client.go
   │   │   └── models.go
   │   └── common/
   │       └── blockchain_client.go  # Common interface
   ```

3. **Application Layer - Factory Pattern**:
   ```go
   type BlockchainServiceFactory struct {
       clients map[string]BlockchainClient
   }

   func (f *BlockchainServiceFactory) GetService(chain string) (*Service, error) {
       client, exists := f.clients[chain]
       if !exists {
           return nil, fmt.Errorf("unsupported chain: %s", chain)
       }
       return NewService(repo, client, config, logger), nil
   }
   ```

4. **Database Schema Evolution**:
   ```sql
   -- Add chain column with partition by chain for performance
   ALTER TABLE delegations ADD COLUMN chain VARCHAR(50);
   CREATE INDEX idx_chain_timestamp ON delegations(chain, timestamp);
   ```

5. **API Changes**:
   ```
   GET /api/v1/{chain}/delegations?year=2023
   GET /api/v1/delegations?chain=tezos,ethereum&year=2023
   ```

This approach maintains clean architecture principles while enabling multi-chain support through composition and abstraction.

#### **Repository Pattern** (postgres/repository.go)
- Batch inserts with transactions
- ON CONFLICT handling for idempotency
- Context timeout management (5s for single, 30s for batch)
- **Trade-off**: Using pgx directly vs. ORM for performance

#### **Rate Limiting** (tzkt/client.go:40)
```go
rateLimiter: rate.NewLimiter(rate.Every(100*time.Millisecond), 10)
```
- Prevents API throttling with 10 requests/second burst
- **Improvement**: Could be configurable based on API tier

### 2. Data Integrity & Consistency

#### **Deduplication Strategy**
- Uses `operation_hash` as unique constraint
- ON CONFLICT DO UPDATE ensures idempotency
- **Question**: Why update on conflict instead of skip?

#### **Sync Verification** (logs line 402-403)
```
"dbCount":2456, "tzktCount":3296, "difference":840, "percentageMissing":"25.49%"
```
- Detects data inconsistencies
- **Issue Found**: 25.49% data missing - needs investigation
- **Solution**: Could implement reconciliation job

#### **Backup System**
- Automatic backup creation post-indexing
- Restore from `backups/latest.sql.gz` on startup
- **Error** (line 409): Missing backup.sh script - needs fixing

### 3. Performance Optimizations

#### **Batch Processing**
- Processes 1,000 delegations per batch
- Transaction-wrapped for atomicity
- Sleep between batches (100ms) to avoid overwhelming DB
- **Metrics**: ~1 second per 1,000 records insert

#### **Connection Pooling**
- Uses pgxpool for efficient connection management
- Configurable pool size (not shown in code sample)
- **Consideration**: Monitor pool exhaustion in production

#### **Concurrent Processing**
- Uses errgroup for managing goroutines (service.go:19)
- Mutex protection for shared state (service.go:31)
- **Enhancement**: Could parallelize TzKT API calls

### 4. Testing Strategy

#### **Test Coverage**
- Unit tests: Service, Domain, HTTP handlers, Logger
- Integration tests: Database operations
- **Skip Pattern**: Integration tests marked as skip in unit test runs
- **Test Results**: All 23 unit tests passing

#### **Test Organization**
```go
// Example: Delegation address validation (domain/delegation_test.go)
- Valid tz1, tz2, tz3, KT1 addresses
- Empty address handling
- Invalid prefix detection
```
- Table-driven tests for comprehensive coverage
- Mock implementations for external dependencies

### 5. Error Handling & Observability

#### **Structured Logging** (logger/logger.go)
```go
logger.Errorw("Failed to fetch delegations",
    "error", err,
    "level", currentLevel)
```
- Uses zap for structured logging
- Different log levels (DEBUG, INFO, WARN, ERROR)
- Contextual information included

#### **Metrics Collection** (metrics/metrics.go)
- Prometheus metrics for API latency
- Total delegations indexed counter
- TzKT API request success rate
- **Missing**: Database query performance metrics

#### **Health Checks**
- `/health` endpoint with DB connectivity check
- `/readiness` for Kubernetes deployments
- Returns delegation count as health indicator

### 6. Potential Improvements

1. **Missing Backup Script**: Line 409 shows backup.sh not found
2. **Data Sync Issue**: 25% data missing needs investigation
3. **Configuration**: Hardcoded values could be environment variables
4. **Retry Logic**: Could implement exponential backoff
5. **Caching**: No caching layer for frequently accessed data
6. **API Versioning**: Currently no version in URL path

---

## 🎨 System Design Exercise Preparation

### Current Architecture
```
[TzKT API] --> [Service] --> [PostgreSQL]
     |             |              |
     v             v              v
[Rate Limiter] [Metrics]    [Backups]
                   |
                   v
            [Prometheus/Grafana]
```

### Scaling Considerations

#### **Horizontal Scaling**
- Service is stateless (except polling state)
- Could use Redis for distributed locking
- Load balancer for multiple instances
- **Challenge**: Coordinate indexing across instances

#### **Database Scaling**
- Read replicas for query load
- Partitioning by timestamp (yearly tables)
- Connection pooling optimization
- **Consider**: TimescaleDB for time-series optimization

#### **Caching Strategy**
```
Client --> Cache --> Service --> Database
            |
            v
         [Redis]
```
- Cache frequently accessed years
- TTL based on data freshness requirements
- Cache invalidation on new delegations

### Advanced System Design Topics

#### **Event-Driven Architecture**
```
[Indexer] --> [Kafka] --> [Processors]
                |
                v
          [Stream Analytics]
```
- Publish delegation events to message queue
- Enable real-time analytics
- Support multiple consumers

#### **Multi-Region Deployment**
- Regional API endpoints
- Cross-region replication
- Edge caching with CDN
- Disaster recovery strategy

#### **Monitoring & Alerting**
```
Service --> Metrics --> Prometheus --> Grafana
         --> Logs   --> ELK Stack
         --> Traces --> Jaeger
```
- SLI/SLO definitions
- Alert thresholds
- Distributed tracing
- Log aggregation

### Possible Design Questions

1. **How would you handle 100x traffic increase?**
   - Implement caching layer
   - Database read replicas
   - API rate limiting per client
   - Horizontal pod autoscaling

2. **How to ensure zero data loss?**
   - Write-ahead logging
   - Multi-region backups
   - Point-in-time recovery
   - Change data capture (CDC)

3. **How to support real-time updates?**
   - WebSocket connections
   - Server-sent events (SSE)
   - GraphQL subscriptions
   - Polling vs. push notifications

4. **How to handle blockchain reorganizations?**
   - Store block confirmations
   - Implement reorg detection
   - Rollback mechanism
   - Finality threshold

---

## 💡 Key Technical Decisions to Highlight

### Why Go?
- Excellent concurrency model (goroutines)
- Strong standard library
- Good performance for I/O-bound operations
- Easy deployment (single binary)

### Why PostgreSQL?
- ACID compliance for financial data
- Rich indexing capabilities
- Mature ecosystem
- JSON support for flexible schemas

### Why Clean Architecture?
- Testability (83% coverage achievable)
- Maintainability
- Framework independence
- Business logic isolation

### Trade-offs Made
1. **Batch size (1000)**: Balance between memory usage and performance
2. **Polling interval (30s)**: Freshness vs. API load
3. **No ORM**: Performance over convenience
4. **Structured logging**: Debugging ease over log size

---

## 🎯 Questions to Ask

### About the Team
- What's the current scale of delegation data?
- What are the main performance bottlenecks?
- How do you handle blockchain reorganizations?
- What's the deployment strategy (K8s, Docker Swarm)?

### About the Role
- What would be the first priority improvements?
- How do you measure service success?
- What's the on-call rotation like?
- How do you handle technical debt?

### About the Architecture
- Are there other blockchain integrations planned?
- What's the data retention policy?
- How do you handle GDPR/compliance?
- What's the disaster recovery plan?

---

## 📝 Final Preparation Checklist

- [ ] Review error handling patterns in the code
- [ ] Understand the indexing algorithm flow
- [ ] Be ready to explain batch processing strategy
- [ ] Practice drawing the system architecture
- [ ] Prepare scaling strategy discussion
- [ ] Think about monitoring improvements
- [ ] Consider security enhancements
- [ ] Review Docker/K8s deployment approach

Remember: The logs show successful operation with good performance metrics. Focus on demonstrating understanding of the architecture, ability to identify improvements, and system design thinking for scale.
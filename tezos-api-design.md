# Tezos API and Data Aggregation Stack - System Design

## 📊 Requirements Analysis

### Scale & Performance Requirements
- **API Traffic**: 50M calls/month = ~19 requests/second average (100+ RPS peak)
- **Data Volume**:
  - 175,000 active delegations
  - 200 new delegations/day
  - Rewards every ~2.84 days (24,576 blocks × 10 sec)
  - Historical data since July 2018 (~6.5 years)
  - **Estimated rewards records**: 175,000 × (365/2.84) × 6.5 = ~146M records

### SLA Requirements
- **Latency**: < 200ms API response time
- **Data Freshness**: < 10 seconds from on-chain to API
- **Availability**: High availability (99.9%+)

---

## 🏗️ System Architecture

### High-Level Architecture
```
                    ┌─────────────────────────────────────┐
                    │         Load Balancer (AWS ALB)      │
                    └────────────┬────────────────────────┘
                                 │
                ┌────────────────┴────────────────────┐
                │                                      │
        ┌───────▼────────┐                  ┌─────────▼────────┐
        │  API Gateway    │                  │   API Gateway    │
        │  (Region 1)     │                  │   (Region 2)     │
        └────────┬────────┘                  └─────────┬────────┘
                 │                                      │
        ┌────────▼────────┐                  ┌─────────▼────────┐
        │  Redis Cache    │◄─────Sync────────►   Redis Cache    │
        │   (Primary)     │                  │   (Replica)      │
        └────────┬────────┘                  └─────────┬────────┘
                 │                                      │
        ┌────────▼────────┐                  ┌─────────▼────────┐
        │  API Services   │                  │  API Services    │
        │  (K8s Cluster)  │                  │  (K8s Cluster)   │
        └────────┬────────┘                  └─────────┬────────┘
                 │                                      │
                 └──────────────┬───────────────────────┘
                                │
                    ┌───────────▼────────────┐
                    │   Aurora PostgreSQL    │
                    │  (Multi-AZ, Read Replicas)│
                    └───────────▲────────────┘
                                │
                    ┌───────────┴────────────┐
                    │   Real-time Indexer    │
                    │   (Kafka + Flink)      │
                    └───────────▲────────────┘
                                │
                    ┌───────────┴────────────┐
                    │   Tezos Node + TzKT    │
                    │      WebSocket          │
                    └─────────────────────────┘
```

---

## 💾 Data Model Design

### PostgreSQL Schema

```sql
-- Operations table (partitioned by timestamp)
CREATE TABLE operations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operation_hash VARCHAR(64) UNIQUE NOT NULL,
    type VARCHAR(20) NOT NULL, -- 'delegation' or 'reward_payment'
    timestamp TIMESTAMPTZ NOT NULL,
    amount NUMERIC(30, 0) NOT NULL,
    delegator VARCHAR(36) NOT NULL,
    baker VARCHAR(36) NOT NULL,
    level BIGINT NOT NULL,
    block_hash VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions
CREATE TABLE operations_2024_01 PARTITION OF operations
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- Indexes for performance
CREATE INDEX idx_operations_delegator_timestamp
    ON operations(delegator, timestamp DESC);
CREATE INDEX idx_operations_baker_timestamp
    ON operations(baker, timestamp DESC);
CREATE INDEX idx_operations_type_timestamp
    ON operations(type, timestamp DESC);

-- Materialized view for daily rewards aggregation
CREATE MATERIALIZED VIEW daily_rewards AS
SELECT
    DATE(timestamp) as date,
    delegator,
    baker,
    SUM(amount) as rewards_amount,
    COUNT(*) as payment_count
FROM operations
WHERE type = 'reward_payment'
GROUP BY DATE(timestamp), delegator, baker
WITH DATA;

CREATE UNIQUE INDEX idx_daily_rewards
    ON daily_rewards(date, delegator, baker);

-- Refresh strategy
CREATE OR REPLACE FUNCTION refresh_daily_rewards()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY daily_rewards;
END;
$$ LANGUAGE plpgsql;
```

---

## 🚀 Real-time Indexing Pipeline

### Components for 10-second SLA

```yaml
# Kafka Topics Configuration
topics:
  - name: tezos.blocks
    partitions: 10
    replication: 3
  - name: tezos.operations.raw
    partitions: 20
    replication: 3
  - name: tezos.operations.processed
    partitions: 20
    replication: 3
```

### Indexer Service (Go)
```go
type IndexerService struct {
    tzktClient    *TzktWebSocketClient
    kafkaProducer *KafkaProducer
    metrics       *PrometheusMetrics
}

func (s *IndexerService) StreamBlocks() {
    // Connect to TzKT WebSocket for real-time updates
    ws := s.tzktClient.Connect("wss://api.tzkt.io/v1/ws")

    // Subscribe to operations
    ws.Subscribe([]string{
        "operations/delegation",
        "operations/endorsement_with_slot",  // For rewards
    })

    for message := range ws.Messages {
        // Process within 1 second
        go s.processOperation(message)
    }
}

func (s *IndexerService) processOperation(msg Message) {
    start := time.Now()
    defer s.metrics.RecordProcessingTime(time.Since(start))

    // Parse and validate
    op := parseOperation(msg)

    // Send to Kafka for processing
    s.kafkaProducer.Send("tezos.operations.raw", op)
}
```

### Stream Processing (Apache Flink)
```java
public class OperationProcessor {
    public static void main(String[] args) {
        StreamExecutionEnvironment env =
            StreamExecutionEnvironment.getExecutionEnvironment();

        // Configure checkpointing for exactly-once semantics
        env.enableCheckpointing(5000);
        env.setParallelism(10);

        // Read from Kafka
        DataStream<Operation> operations = env
            .addSource(new FlinkKafkaConsumer<>(
                "tezos.operations.raw",
                new OperationDeserializer(),
                kafkaProps))
            .name("Kafka Source");

        // Process and enrich
        DataStream<EnrichedOperation> enriched = operations
            .keyBy(op -> op.getDelegator())
            .process(new OperationEnricher())
            .name("Enrich Operations");

        // Write to PostgreSQL with batching
        enriched.addSink(new PostgresSink())
            .name("PostgreSQL Sink");

        // Also write to cache
        enriched.addSink(new RedisSink())
            .name("Redis Cache Update");

        env.execute("Tezos Operation Processor");
    }
}
```

---

## 🎯 API Service Implementation

### API Handlers with Caching

```go
type APIService struct {
    db          *pgxpool.Pool
    cache       *redis.Client
    metrics     *Metrics
}

// GET /xtz/operations
func (s *APIService) GetOperations(c *gin.Context) {
    ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Millisecond)
    defer cancel()

    // Try cache first
    cacheKey := fmt.Sprintf("ops:%s", c.Request.URL.Query().Encode())
    if cached, err := s.cache.Get(ctx, cacheKey).Result(); err == nil {
        c.Data(200, "application/json", []byte(cached))
        return
    }

    // Query with timeout budget
    query := `
        SELECT timestamp, amount, delegator, baker, level, type
        FROM operations
        WHERE timestamp > NOW() - INTERVAL '7 days'
        ORDER BY timestamp DESC
        LIMIT 1000
    `

    rows, err := s.db.Query(ctx, query)
    if err != nil {
        c.JSON(500, gin.H{"error": "database error"})
        return
    }

    // Process and cache
    result := processRows(rows)
    s.cache.Set(ctx, cacheKey, result, 30*time.Second)

    c.JSON(200, gin.H{"data": result})
}

// GET /xtz/rewards
func (s *APIService) GetRewards(c *gin.Context) {
    ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Millisecond)
    defer cancel()

    delegator := c.Query("delegator")
    baker := c.Query("baker")

    // Use materialized view for aggregated data
    query := `
        SELECT date, SUM(rewards_amount) as rewards_amount
        FROM daily_rewards
        WHERE ($1::text IS NULL OR delegator = $1)
          AND ($2::text IS NULL OR baker = $2)
          AND date > NOW() - INTERVAL '30 days'
        GROUP BY date
        ORDER BY date DESC
    `

    rows, err := s.db.Query(ctx, query, delegator, baker)
    if err != nil {
        c.JSON(500, gin.H{"error": "database error"})
        return
    }

    result := processRewardRows(rows)
    c.JSON(200, gin.H{"data": result})
}
```

---

## 🔄 Caching Strategy

### Multi-Layer Cache Architecture

```yaml
# Redis Configuration
redis:
  cluster: true
  nodes: 6
  memory: 16GB per node
  eviction_policy: allkeys-lru

cache_layers:
  L1_Local:
    type: in-memory
    size: 1GB
    ttl: 10s

  L2_Redis:
    type: redis-cluster
    ttl: 60s
    patterns:
      - operations:* (30s TTL)
      - rewards:* (300s TTL)
      - delegator:* (60s TTL)

  L3_CDN:
    type: CloudFront
    ttl: 300s
    invalidation: on-write
```

### Cache Warming Strategy
```go
func WarmCache() {
    // Pre-load frequently accessed data
    popularDelegators := getTopDelegators(100)

    for _, delegator := range popularDelegators {
        // Warm operations cache
        operations := fetchOperations(delegator)
        cache.Set(fmt.Sprintf("ops:delegator:%s", delegator), operations, 5*time.Minute)

        // Warm rewards cache
        rewards := fetchRewards(delegator)
        cache.Set(fmt.Sprintf("rewards:delegator:%s", delegator), rewards, 10*time.Minute)
    }
}
```

---

## 📈 Scaling & Performance Optimizations

### Database Optimizations

1. **Partitioning Strategy**:
   - Partition by month for operations table
   - Automatic partition management
   - Archive old partitions to S3

2. **Read Replicas**:
   - 3 read replicas for query distribution
   - Geo-distributed for regional performance

3. **Connection Pooling**:
   ```yaml
   pgbouncer:
     pool_mode: transaction
     max_client_conn: 1000
     default_pool_size: 25
   ```

### API Scaling

```yaml
# Kubernetes HPA Configuration
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: api-service-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-service
  minReplicas: 10
  maxReplicas: 100
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: http_requests_per_second
      target:
        type: AverageValue
        averageValue: "1000"
```

---

## 🛡️ High Availability Design

### Multi-Region Deployment

```yaml
regions:
  primary:
    location: us-east-1
    components:
      - API Gateway (Active)
      - Redis Cluster (Primary)
      - Aurora Writer Instance
      - Kafka Cluster
      - Indexer Services

  secondary:
    location: eu-west-1
    components:
      - API Gateway (Active)
      - Redis Cluster (Replica)
      - Aurora Read Replicas
      - Kafka Mirror
      - Standby Indexer

  disaster_recovery:
    location: ap-southeast-1
    components:
      - Cold standby
      - S3 backup storage
      - Database snapshots
```

### Failover Strategy

1. **Database Failover**: Aurora automatic failover (< 30 seconds)
2. **Cache Failover**: Redis Sentinel for automatic promotion
3. **API Failover**: DNS-based with health checks (< 60 seconds)
4. **Indexer Failover**: Kafka consumer groups with automatic rebalancing

---

## 📊 Monitoring & Observability

### Key Metrics

```yaml
SLIs:
  - API Latency P99 < 200ms
  - Data Freshness < 10 seconds
  - Error Rate < 0.1%
  - Availability > 99.9%

Alerts:
  - API latency > 150ms (warning)
  - API latency > 200ms (critical)
  - Indexing lag > 5 seconds (warning)
  - Indexing lag > 10 seconds (critical)
  - Error rate > 0.5% (warning)
  - Database CPU > 80% (warning)
  - Cache hit rate < 80% (warning)
```

### Monitoring Stack

```
Metrics: Prometheus + Grafana
Logs: ELK Stack (Elasticsearch, Logstash, Kibana)
Tracing: Jaeger
APM: DataDog
Alerting: PagerDuty
```

---

## 💰 Cost Optimization

### Estimated Monthly Costs (AWS)

```yaml
Infrastructure:
  API_Servers: $2,000 (10x c5.2xlarge reserved)
  Database: $3,000 (Aurora PostgreSQL, Multi-AZ)
  Cache: $1,500 (ElastiCache Redis Cluster)
  Kafka: $2,000 (MSK Cluster)
  Load_Balancer: $500
  Data_Transfer: $1,000
  Monitoring: $500

Total: ~$10,500/month

Cost_Per_Request: $0.00021 (well within budget)
```

### Optimization Strategies

1. **Reserved Instances**: 40% cost reduction
2. **Spot Instances**: For batch processing
3. **S3 Intelligent Tiering**: For historical data
4. **CloudFront**: Reduce egress costs
5. **Graviton Instances**: 20% better price/performance

---

## 🚦 Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)
- Set up PostgreSQL with partitioning
- Deploy Kafka cluster
- Implement basic indexer

### Phase 2: Real-time Pipeline (Week 3-4)
- WebSocket connection to TzKT
- Flink stream processing
- < 10 second indexing

### Phase 3: API Development (Week 5-6)
- Implement /xtz/operations endpoint
- Implement /xtz/rewards endpoint
- Add caching layer

### Phase 4: High Availability (Week 7-8)
- Multi-region deployment
- Failover testing
- Load testing (target: 1000 RPS)

### Phase 5: Optimization (Week 9-10)
- Performance tuning
- Cost optimization
- Documentation

---

## 🔑 Key Design Decisions

1. **PostgreSQL over NoSQL**: ACID guarantees for financial data
2. **Kafka for Streaming**: Guaranteed delivery, replay capability
3. **Materialized Views**: Pre-aggregated data for rewards endpoint
4. **Multi-layer Caching**: Balance between freshness and performance
5. **Partitioning Strategy**: Manageable table sizes, faster queries
6. **WebSocket Indexing**: Real-time data without polling overhead

This design handles 50M requests/month with sub-200ms latency while maintaining 10-second data freshness and high availability.
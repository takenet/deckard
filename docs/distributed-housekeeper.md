# Distributed Housekeeper Execution

This document describes the distributed execution feature for Deckard's housekeeper, which allows running multiple housekeeper instances simultaneously without conflicts.

## Overview

The distributed housekeeper execution feature enables:
- **Task coordination**: Distributed locks ensure only one instance runs each task at a time
- **Metrics leader election**: Prevents duplication of Prometheus metrics across instances  
- **Performance improvements**: Parallel processing for unlock operations
- **Scalability**: Multiple housekeeper pods can be deployed for better availability

## Configuration

### Environment Variables

```bash
# Enable distributed execution
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true

# Set unique instance ID (optional - auto-generated if not provided)
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID=housekeeper-pod-1

# Set lock TTL (default: 30s)
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_LOCK_TTL=30s

# Set unlock parallelism (default: 5)
DECKARD_HOUSEKEEPER_UNLOCK_PARALLELISM=10
```

### Configuration File (YAML)

```yaml
housekeeper:
  distributed_execution:
    enabled: true
    instance_id: "housekeeper-pod-1"  # Optional
    lock_ttl: "30s"
  unlock:
    parallelism: 10
```

## How It Works

### Task Coordination

When distributed execution is enabled, each housekeeper task acquires a distributed lock before execution:

1. **UNLOCK**: Processes locked messages with parallel workers
2. **TIMEOUT**: Handles message timeouts 
3. **RECOVERY**: Recovers messages from storage to cache (critical task)
4. **TTL**: Removes expired messages (critical task)
5. **MAX_ELEMENTS**: Removes exceeding messages (critical task)
6. **METRICS**: Computes queue metrics (leader-only)

### Metrics Leader Election

Only one housekeeper instance computes and exposes metrics to prevent duplication:
- Metrics leader is elected using a distributed lock
- Non-leader instances skip metrics computation
- Leader lock has extended TTL for stability

### Lock Management

- **Lock Key Format**: `deckard:housekeeper:lock:{task_name}`
- **Instance Identification**: Each instance has a unique ID
- **TTL Management**: Locks auto-expire to prevent deadlocks
- **Error Handling**: Failed lock acquisitions are logged and retried

## Deployment Examples

### Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deckard-housekeeper
spec:
  replicas: 3  # Multiple instances for availability
  selector:
    matchLabels:
      app: deckard-housekeeper
  template:
    metadata:
      labels:
        app: deckard-housekeeper
    spec:
      containers:
      - name: deckard
        image: deckard:latest
        env:
        - name: DECKARD_HOUSEKEEPER_ENABLED
          value: "true"
        - name: DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED
          value: "true"
        - name: DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name  # Use pod name as instance ID
        - name: DECKARD_REDIS_ADDRESS
          value: "redis-service"
        - name: DECKARD_REDIS_PORT
          value: "6379"
```

### Docker Compose

```yaml
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    
  deckard-hk-1:
    image: deckard:latest
    environment:
      - DECKARD_HOUSEKEEPER_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID=hk-1
      - DECKARD_REDIS_ADDRESS=redis
    depends_on:
      - redis
      
  deckard-hk-2:
    image: deckard:latest
    environment:
      - DECKARD_HOUSEKEEPER_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID=hk-2
      - DECKARD_REDIS_ADDRESS=redis
    depends_on:
      - redis
```

## Monitoring

### Logs

Distributed execution adds debug logs for:
- Lock acquisition/release
- Task skipping when locks are held by other instances
- Metrics leader election

Example log messages:
```
[DEBUG] Acquired distributed lock timeout by instance hk-1
[DEBUG] Skipping task recovery - already running on another instance
[INFO] Instance hk-1 elected as metrics leader
```

### Metrics

Existing housekeeper metrics continue to work:
- Only the metrics leader reports queue metrics
- Task execution metrics include instance information
- Lock-related errors are logged but don't affect existing metrics

## Performance Improvements

### Parallel Unlocking

The unlock task now processes queues in parallel:
- Configurable parallelism level (default: 5 workers)
- Maintains all existing unlock logic and metrics
- Graceful shutdown handling
- Per-queue error isolation

Performance benefits:
- Faster processing when many queues have locked messages
- Better resource utilization
- Reduced lock time precision impact

## Backward Compatibility

When distributed execution is disabled (default):
- Uses NoOp distributed locks (always succeed)
- Maintains original single-instance behavior
- No performance impact
- All existing functionality preserved

## Troubleshooting

### Common Issues

1. **Redis Connection Issues**
   - Ensure Redis is accessible from all housekeeper instances
   - Check Redis authentication and network connectivity

2. **Lock Contention**
   - Increase lock TTL if tasks take longer than expected
   - Monitor logs for lock acquisition failures

3. **Metrics Duplication**
   - Verify only one instance is elected as metrics leader
   - Check distributed execution is enabled on all instances

### Debugging

Enable debug logging to see distributed lock operations:
```bash
DECKARD_LOG_LEVEL=debug
```

Check lock status in Redis:
```bash
redis-cli KEYS "deckard:housekeeper:lock:*"
```
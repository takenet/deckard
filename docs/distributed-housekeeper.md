# Distributed Housekeeper Execution

This document describes the distributed execution feature for Deckard's housekeeper, which allows running multiple housekeeper instances simultaneously without conflicts.

## Overview

The distributed housekeeper execution feature enables:
- **Task coordination**: A generic distributed lock ([`internal/lock`](../internal/lock)) ensures only one instance runs a given task at a time
- **Leader election**: A generic leader election module ([`internal/election`](../internal/election)), built on top of `internal/lock`, elects a single housekeeper leader fleet-wide, responsible for tasks that must not run concurrently or be duplicated (metrics computation and cache recovery)
- **Performance improvements**: Parallel processing for unlock operations
- **Scalability**: Multiple housekeeper pods (or gRPC pods with an embedded housekeeper) can be deployed for better availability

Both modules are backend-agnostic: they only depend on a small `Store` interface (`SetNX`/`CompareAndDelete`/`CompareAndExpire`), which `queue/cache.Cache` already satisfies structurally. Redis is the first (and currently only) backend, but a future non-Redis backend (e.g. Postgres advisory locks, etcd, a Kubernetes `Lease`) only needs to implement `lock.Store` - neither `internal/lock` nor `internal/election` need to change.

## Task Classification

| Task | Category | Coordination |
| --- | --- | --- |
| `UNLOCK` | Any instance | Per-task distributed lock (atomic, first-come-first-served) |
| `TIMEOUT` | Any instance | Per-task distributed lock |
| `TTL` | Any instance | Per-task distributed lock |
| `MAX_ELEMENTS` | Any instance | Per-task distributed lock |
| `METRICS` | Leader only | Leader election + per-task distributed lock (defense-in-depth) |
| `RECOVERY` | Leader only | Leader election + per-task distributed lock (defense-in-depth) |

`METRICS` and `RECOVERY` are restricted to the elected leader because they operate on fleet-wide state: `METRICS` exposes gauges (queue size, oldest element, etc.) that must not be duplicated across instances, and `RECOVERY` reconstructs the entire cache from storage, which must not run concurrently from multiple instances. The other tasks are idempotent and safe on any instance, so they only need mutual exclusion (via the same per-task lock) to avoid wasted duplicate work.

## Configuration

### Environment Variables

```bash
# Enable distributed execution (locks + leader election)
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true

# Set unique instance ID (optional - defaults to hostname, which is the pod
# name in Kubernetes, already unique across deployments)
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID=housekeeper-pod-1

# TTL for the per-task atomic locks (UNLOCK, TIMEOUT, TTL, MAX_ELEMENTS, and the
# defense-in-depth lock used by METRICS/RECOVERY). Default: 30s
DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_LOCK_TTL=30s

# Lease TTL for the leader election. The election's background renewal loop
# runs at lease_ttl/3. Default: 15s
DECKARD_HOUSEKEEPER_ELECTION_LEASE_TTL=15s

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
  election:
    lease_ttl: "15s"
  unlock:
    parallelism: 10
```

## How It Works

### Task Coordination (`internal/lock`)

`internal/lock` defines a generic `Locker` interface (`TryAcquire`/`Release`/`Renew`) with a Redis-backed implementation and a no-op implementation for single-instance mode. Every atomic task acquires a named lock (`housekeeper:lock:{task_name}`) before executing and releases it right after, so at most one instance runs a given task at a time.

### Leader Election (`internal/election`)

`internal/election` defines a generic `Elector` interface (`Start`/`Stop`/`IsLeader`/`ID`) with two implementations:
- `LeaseElector`: campaigns for a single well-known lock (`housekeeper:election:leader`) in a background goroutine, independent of any task's own schedule. While follower, it periodically tries to acquire the lease; while leader, it periodically renews it. If renewal fails (e.g. the process was slow or partitioned from Redis long enough for the lease to expire and be taken over), it demotes itself to follower.
- `StaticElector`: always reports itself as leader, used when distributed execution is disabled (single-instance mode).

The election loop runs on its own ticker (`lease_ttl/3`), decoupled from the metrics or recovery task delays, so changing those delays doesn't affect leadership stability. On graceful shutdown, the elector releases leadership immediately instead of waiting for the lease to expire, so failover to another instance is fast.

### Lock and Election Key Format

- Per-task atomic lock: logical name `housekeeper:lock:{task_name}` (e.g. `housekeeper:lock:unlock`)
- Leader election lock: logical name `housekeeper:election:leader`
- These logical names are the raw keys `internal/lock`/`internal/election` pass to the backing `cache.Cache`. In production (Redis), `RedisCache.generalCacheKey` wraps every such key with the deployment's configured cache prefix and a hash-tag: `<prefix>:{<logical_name>}`. With the default prefix (`deckard_v1`), the lock for the `timeout` task is actually stored as `deckard_v1:{housekeeper:lock:timeout}`, and the leader election lock as `deckard_v1:{housekeeper:election:leader}`.
- **Instance Identification**: Each instance has a unique ID (defaults to hostname)
- **TTL Management**: Locks and the election lease auto-expire to prevent deadlocks
- **Error Handling**: Failed lock/election acquisitions are logged and retried on the next cycle

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
        - name: DECKARD_CACHE_TYPE
          value: "REDIS"
        - name: DECKARD_CACHE_URI
          value: "redis://redis-service:6379/0"
```

This same election/lock namespace is also shared by any gRPC pods that embed the housekeeper (`housekeeper.self.enabled=true` in the Helm chart), since they connect to the same Redis instance - exactly one leader is elected across the whole fleet regardless of how many deployments run the housekeeper.

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
      - DECKARD_CACHE_TYPE=REDIS
      - DECKARD_CACHE_URI=redis://redis:6379/0
    depends_on:
      - redis

  deckard-hk-2:
    image: deckard:latest
    environment:
      - DECKARD_HOUSEKEEPER_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true
      - DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_INSTANCE_ID=hk-2
      - DECKARD_CACHE_TYPE=REDIS
      - DECKARD_CACHE_URI=redis://redis:6379/0
    depends_on:
      - redis
```

## Monitoring

### Logs

Distributed execution adds debug/info logs for:
- Lock acquisition/release for atomic tasks
- Task skipping when a lock is held by another instance
- Leadership transitions (elected / lost leadership)

Example log messages:
```
[DEBUG] Acquired lock housekeeper:lock:timeout by owner hk-1
[DEBUG] Skipping task recovery - already running on another instance
[INFO] Instance hk-1 elected as leader for housekeeper:election:leader
[INFO] Instance hk-1 lost leadership for housekeeper:election:leader: cannot renew lock housekeeper:election:leader - not held by this owner
```

### Metrics

- `deckard_housekeeper_leader`: gauge, `1` if this instance currently holds housekeeper leadership, `0` otherwise. Useful to confirm exactly one instance is leader at any time (issue [#21](https://github.com/takenet/deckard/issues/21)'s metrics-duplication concern), and to observe failover during deploys/restarts.
- `deckard_housekeeper_task_latency`: existing histogram, now recorded for every task regardless of category.
- `deckard_oldest_message` / `deckard_total_messages`: existing gauges, now guaranteed to be reported by exactly the leader instance.

## Performance Improvements

### Parallel Unlocking

The unlock task processes queues in parallel:
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
- Uses `lock.NewNoopLocker()` (always succeeds) for atomic tasks
- Uses `election.NewStaticElector()` (always leader) for leader tasks
- Maintains original single-instance behavior
- No performance impact, no Redis calls for locking/election
- All existing functionality preserved

## Troubleshooting

### Common Issues

1. **Redis Connection Issues**
   - Ensure Redis is accessible from all housekeeper instances
   - Check `DECKARD_CACHE_URI` credentials and network connectivity

2. **Lock Contention**
   - Increase `DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_LOCK_TTL` if atomic tasks take longer than expected
   - Monitor logs for lock acquisition failures

3. **No leader elected / metrics missing**
   - Check `deckard_housekeeper_leader` across instances - exactly one should report `1`
   - Verify `DECKARD_HOUSEKEEPER_DISTRIBUTED_EXECUTION_ENABLED=true` on all instances sharing the same Redis
   - Increase `DECKARD_HOUSEKEEPER_ELECTION_LEASE_TTL` if leadership flaps under Redis latency spikes

### Debugging

Enable debug logging to see lock and election operations:

```bash
DECKARD_LOG_LEVEL=debug
```

Check lock/election keys in Redis:
```bash
redis-cli KEYS "*housekeeper:lock:*"
redis-cli GET "*housekeeper:election:leader*"
```
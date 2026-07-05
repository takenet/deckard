# Redis Cluster Configuration

This document explains how to configure Deckard to work with Redis Clusters.

## Overview

Deckard now supports both single-node Redis and Redis Cluster deployments. When Redis Cluster mode is enabled, Deckard uses hash tags in key naming to ensure all queue-related keys land on the same cluster slot, enabling atomic operations via Lua scripts.

## Configuration

### Single Node Redis (Default)

This is the traditional configuration that works with a single Redis instance:

```yaml
cache:
  type: REDIS
redis:
  address: localhost
  port: 6379
  password: ""  # optional
  db: 0         # optional
```

Environment variables:
```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_REDIS_ADDRESS=localhost
DECKARD_REDIS_PORT=6379
DECKARD_REDIS_PASSWORD=mypassword  # optional
DECKARD_REDIS_DB=0                 # optional
```

### Redis Cluster

To enable Redis Cluster support:

```yaml
cache:
  type: REDIS
redis:
  cluster:
    mode: true
    addresses: redis-node-1:6379,redis-node-2:6379,redis-node-3:6379
```

Note: `redis.cluster.addresses` is parsed as a comma-separated string (same format used by `DECKARD_REDIS_CLUSTER_ADDRESSES`).

Environment variables:
```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_REDIS_CLUSTER_MODE=true
DECKARD_REDIS_CLUSTER_ADDRESSES=redis-node-1:6379,redis-node-2:6379,redis-node-3:6379
DECKARD_REDIS_PASSWORD=mypassword  # optional, if cluster requires auth
```

## Key Naming Differences

### Single Node Redis
Keys are named without hash tags:
- `deckard:queue:clusterhash`
- `deckard:queue:clusterhash:tmp`
- `deckard:queue:clusterhash:lock_ack`

### Redis Cluster  
Keys use hash tags to ensure co-location:
- `deckard:queue:{clusterhash}`
- `deckard:queue:{clusterhash}:tmp`
- `deckard:queue:{clusterhash}:lock_ack`

The `{clusterhash}` hash tag ensures all keys for the same queue hash to the same cluster slot.

## Migration from Single Node to Cluster

Single-node keys (`deckard:queue:myqueue`) and cluster-mode keys (`deckard:queue:{myqueue}`) are
literally different key strings, not just differently-atomic. This means an **in-place migration
is not supported**: pointing a cluster-mode Deckard instance at the same Redis data (via RDB
restore, `redis-cli --cluster import`, replication + resharding, etc.) will not work - the new
instance won't find any of the old keys and every queue will appear empty, while the old keys are
left behind as orphaned data.

Because Redis here is a rebuildable cache/index over the durable `storage` backend (not the source
of truth for message content), the supported migration path is:

1. Provision a fresh, empty Redis Cluster.
2. Deploy Deckard pointed at it with `redis.cluster.mode: true`.
3. Let the housekeeper's existing recovery task (`RecoveryMessagesPool`) rebuild the active, lock,
   and processing pools directly from `storage` - no manual key migration needed.
4. Note: any message that was mid-flight (locked, awaiting ack/nack) at cutover time loses its lock
   state and becomes immediately deliverable again, consistent with Deckard's at-least-once
   semantics. Plan the cutover during a quiet window if duplicate delivery during the switch is a
   concern.
5. Decommission the old single-node Redis once queue sizes are verified against `storage`.

## Docker Compose Example

The repository's [`docker/docker-compose.yml`](../docker/docker-compose.yml) already ships a
6-node Redis Cluster (3 masters + 3 replicas) for local development and testing:
`redis-cluster-node-0` through `redis-cluster-node-5`, plus a `redis-cluster-init` helper service
that forms the cluster. All cluster node containers (and the init helper) run with
`network_mode: host` and `--cluster-announce-ip 127.0.0.1`, so the cluster is reachable at
`localhost:7000`-`localhost:7005` from processes running directly on the host (not just from other
containers) - this is what makes it usable directly by `go test` and by CI runners, not only by
other docker-compose services.

Bring the cluster up with:

```bash
docker compose -f docker/docker-compose.yml up -d \
  redis-cluster-node-0 redis-cluster-node-1 redis-cluster-node-2 \
  redis-cluster-node-3 redis-cluster-node-4 redis-cluster-node-5

docker compose -f docker/docker-compose.yml run --rm redis-cluster-init
```

Point Deckard at it with:
```bash
DECKARD_REDIS_CLUSTER_MODE=true
DECKARD_REDIS_CLUSTER_ADDRESSES=localhost:7000,localhost:7001,localhost:7002
```

Tear it down with:
```bash
docker compose -f docker/docker-compose.yml down -v
```

## Testing

Cluster integration tests (everything matching `-run Cluster` in
`internal/queue/cache/redis_cache_cluster_test.go`) connect to the cluster started above at
`localhost:7000-7002` and run as part of the normal integration test suite (`make test` /
`make integration-test`) - there is no separate opt-in flag. Bring the cluster up first (see
above), then run:

```bash
go test -v ./internal/queue/cache/ -run Cluster
```

The same `CacheIntegrationTestSuite` used for single-node Redis and the in-memory cache also runs
against the cluster (via `TestRedisCacheClusterIntegration`), so any new cache behavior test
automatically gets cluster coverage without needing a separate cluster-specific copy.

## Troubleshooting

### Common Issues

**1. "redis.cluster.addresses must be specified" error**
- Ensure `DECKARD_REDIS_CLUSTER_ADDRESSES` is set when cluster mode is enabled

**2. "CROSSSLOT Keys in request don't hash to the same slot" error**
- This indicates the hash tag implementation may have an issue
- Verify all keys for a queue use the same hash tag format

**3. Connection timeouts to cluster**
- Ensure all cluster nodes are accessible from Deckard
- Check network connectivity and firewall rules
- Verify cluster is properly initialized

### Performance Considerations

- Redis Cluster may have slightly higher latency than single-node due to network hops
- Hash tags ensure optimal key distribution while maintaining atomicity
- Monitor cluster node resource usage and scale as needed

### Best Practices

1. **Always use odd number of master nodes** (3, 5, 7) for proper quorum
2. **Set up Redis Cluster with replicas** for high availability
3. **Monitor cluster health** using Redis Cluster commands
4. **Test failover scenarios** in staging environments
5. **Use consistent hashing strategy** across all deployments
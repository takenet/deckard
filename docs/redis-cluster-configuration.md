# Redis Cluster Configuration

This document explains how to configure Deckard to work with Redis Clusters.

## Overview

Deckard supports both single-node Redis and Redis Cluster deployments. When Redis Cluster mode is enabled, Deckard uses hash tags in key naming to ensure all queue-related keys land on the same cluster slot, enabling atomic operations via Lua scripts.

## Configuration

Redis connection details (address, credentials, database, TLS) are configured through
`DECKARD_CACHE_URI`.

### Single Node Redis

Single-node Redis is still supported, but the configuration format changed in this PR: Redis deployments now use `DECKARD_CACHE_URI`. Existing standalone deployments that previously relied on separate address/port/password/db settings must migrate to the URI-based format.

This is the required standalone configuration format:

```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_CACHE_URI=redis://localhost:6379/0
```

With authentication and TLS:

```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_CACHE_URI=rediss://user:pass@localhost:6380/0
```

### Redis Cluster

To enable Redis Cluster support, set `DECKARD_REDIS_CLUSTER_MODE=true` and provide a cluster URL
in `DECKARD_CACHE_URI`, using go-redis's own cluster URL format (parsed by
[`redis.ParseClusterURL`](https://pkg.go.dev/github.com/redis/go-redis/v9#ParseClusterURL)): a
base URI for the first seed node, with the remaining seed nodes appended as repeated
`addr=host:port` query parameters. Credentials, TLS and connection tuning options all belong to
the base URI and apply to every node:

```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_REDIS_CLUSTER_MODE=true
DECKARD_CACHE_URI=redis://redis-node-1:6379?addr=redis-node-2:6379&addr=redis-node-3:6379
```

With authentication and TLS:

```bash
DECKARD_CACHE_TYPE=REDIS
DECKARD_REDIS_CLUSTER_MODE=true
DECKARD_CACHE_URI=rediss://user:pass@redis-node-1:6379?addr=redis-node-2:6379&addr=redis-node-3:6379
```

Two Redis Cluster limitations to be aware of:

- There is no database selector: Redis Cluster does not support `SELECT` and always operates on database 0.
- `skip_verify` (insecure TLS, used for standalone above) is not accepted by the cluster URL parser.

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

In addition to the key-format differences below, note that this PR also changes the Redis configuration model for standalone deployments. If you are upgrading from an older Deckard version, migrate your standalone Redis settings to `DECKARD_CACHE_URI` before or during rollout.

Single-node keys (`deckard:queue:myqueue`) and cluster-mode keys (`deckard:queue:{myqueue}`) are literally different key strings, not just differently-atomic. This means an **in-place migration is not supported**: pointing a cluster-mode Deckard instance at the same Redis data (via RDB restore, `redis-cli --cluster import`, replication + resharding, etc.) will not work - the new instance won't find any of the old keys and every queue will appear empty, while the old keys are left behind as orphaned data.

Because Redis here is a rebuildable cache/index over the durable `storage` backend (not the source
of truth for message content), the supported migration path is:

1. Provision a fresh, empty Redis Cluster.
2. Deploy Deckard pointed at it with `redis.cluster.mode: true`.
3. Let the housekeeper's existing recovery task (`RecoveryMessagesPool`) rebuild the active, lock,
   and processing pools directly from `storage` - no manual key migration needed.
4. Note: any message that was mid-flight (locked, awaiting ack/nack) at cutover time loses its lock state and becomes immediately deliverable again, consistent with Deckard's at-least-once semantics. Plan the cutover during a quiet window if duplicate delivery during the switch is a concern.
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

Bring the cluster and a Deckard instance already configured for Redis Cluster up with:

```bash
docker compose -f docker/docker-compose.yml \
  --profile deckard-cluster up -d
```

This starts:

- `redis-cluster-node-0` through `redis-cluster-node-5`
- `redis-cluster-init`
- `deckard-redis-cluster`
- required dependencies (`mongodb`)

Deckard gRPC will be available on the configured cluster port (by default `8082`).

Tear it down with:

```bash
docker compose -f docker/docker-compose.yml down -v
```

## Testing

Cluster integration tests (everything matching `-run Cluster` in
`internal/queue/cache/redis_cache_cluster_test.go`) connect to the cluster started above at `localhost:7000-7002` and run as part of the normal integration test suite (`make test` / `make integration-test`) - there is no separate opt-in flag. Bring the cluster up first (see above), then run:

```bash
go test -v ./internal/queue/cache/ -run Cluster
```

## Troubleshooting

### Common Issues

**1. "cache.uri (DECKARD_CACHE_URI) is required when cache.type is REDIS" error**

- Ensure `DECKARD_CACHE_URI` is set to a cluster URL (base URI plus `addr=` query parameters) when cluster mode is enabled

**2. "CROSSSLOT Keys in request don't hash to the same slot" error**

- This indicates the hash tag implementation may have an issue
- Verify all keys for a queue use the same hash tag format

**3. Connection timeouts to cluster**

- Ensure all cluster nodes are accessible from Deckard
- Check network connectivity and firewall rules
- Verify cluster is properly initialized

**4. Redis nodes data is corrupted, inconsistent or lost**

- Rebuild cache by clearing Redis and letting Deckard repopulate from storage by simply flushing all keys in Redis. No Deckard restart is needed, it will automatically repopulate the cache from storage.

### Performance Considerations

- Redis Cluster may have slightly higher latency than single-node due to network hops
- Hash tags ensure optimal key distribution while maintaining atomicity
- Monitor cluster node resource usage and scale as needed

### Best Practices

1. **Always use odd number of master nodes** (3, 5, 7) for proper quorum
2. **Set up Redis Cluster with replicas** for high availability
3. **Monitor cluster health** using Redis Cluster commands
4. **Test failover scenarios** in staging environments
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
    addresses:
      - redis-node-1:6379
      - redis-node-2:6379
      - redis-node-3:6379
```

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
- `deckard:queue:myqueue`
- `deckard:queue:myqueue:tmp`
- `deckard:queue:myqueue:lock_ack`

### Redis Cluster  
Keys use hash tags to ensure co-location:
- `deckard:queue:{myqueue}`
- `deckard:queue:{myqueue}:tmp`
- `deckard:queue:{myqueue}:lock_ack`

The `{myqueue}` hash tag ensures all keys for the same queue hash to the same cluster slot.

## Migration from Single Node to Cluster

### Option 1: Fresh Deployment
1. Deploy new Deckard instance with cluster configuration
2. Migrate data using your preferred method
3. Switch traffic to new deployment

### Option 2: In-place Migration (Advanced)
⚠️ **Warning**: This approach requires careful planning and testing.

1. Ensure your Redis cluster is properly configured
2. Update Deckard configuration to enable cluster mode
3. Restart Deckard instance

Note: Existing keys will remain in the old format and will need manual migration if cross-key atomicity is required.

## Docker Compose Example

```yaml
version: "3.8"

services:
  deckard:
    image: blipai/deckard:latest
    environment:
      - DECKARD_CACHE_TYPE=REDIS
      - DECKARD_REDIS_CLUSTER_MODE=true
      - DECKARD_REDIS_CLUSTER_ADDRESSES=redis-node-1:6379,redis-node-2:6379,redis-node-3:6379
    depends_on:
      - redis-node-1
      - redis-node-2
      - redis-node-3

  redis-node-1:
    image: redis:7.0
    command: redis-server --port 6379 --cluster-enabled yes --cluster-config-file nodes.conf --cluster-node-timeout 5000
    ports:
      - "6379:6379"

  redis-node-2:
    image: redis:7.0
    command: redis-server --port 6379 --cluster-enabled yes --cluster-config-file nodes.conf --cluster-node-timeout 5000
    ports:
      - "6380:6379"

  redis-node-3:
    image: redis:7.0
    command: redis-server --port 6379 --cluster-enabled yes --cluster-config-file nodes.conf --cluster-node-timeout 5000
    ports:
      - "6381:6379"
```

## Testing

### Local Development with Docker
Use the provided script to set up a local Redis cluster:

```bash
./scripts/setup-redis-cluster.sh
```

### Running Cluster Tests
```bash
export REDIS_CLUSTER_TEST=1
go test -v ./internal/queue/cache/ -run Cluster
```

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
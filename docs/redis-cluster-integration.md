# Redis Cluster Integration Plan

## Overview

This document outlines the plan to improve Deckard's integration with Redis Clusters while maintaining backward compatibility with single-node Redis deployments.

## Problem Statement

Deckard currently uses Lua scripts for atomic operations on Redis, but these scripts access multiple keys that may hash to different slots in a Redis Cluster, causing the scripts to fail. Redis Cluster requires all keys accessed by a Lua script to be on the same hash slot.

## Current Key Naming Pattern

For a queue named `myqueue`, Deckard creates these keys:
- `deckard:queue:myqueue` (active pool)
- `deckard:queue:myqueue:tmp` (processing pool)
- `deckard:queue:myqueue:ack` (ack lock pool)  
- `deckard:queue:myqueue:nack` (nack lock pool)
- `deckard:queue:myqueue:ack:score` (ack score pool)
- `deckard:queue:myqueue:nack:score` (nack score pool)

## Redis Cluster Constraints

1. All keys in a Lua script must hash to the same slot
2. Keys must use hash tags to control slot assignment
3. Scripts can only access keys explicitly provided as KEYS arguments
4. No programmatically-generated key names in scripts

## Solution: Hash Tags

Modify key naming to use Redis hash tags, ensuring all queue-related keys hash to the same slot:

### New Key Naming Pattern (Cluster-Compatible)
- `deckard:queue:{myqueue}` (active pool)
- `deckard:queue:{myqueue}:tmp` (processing pool) 
- `deckard:queue:{myqueue}:ack` (ack lock pool)
- `deckard:queue:{myqueue}:nack` (nack lock pool)
- `deckard:queue:{myqueue}:ack:score` (ack score pool)
- `deckard:queue:{myqueue}:nack:score` (nack score pool)

The `{myqueue}` hash tag ensures all keys for the same queue land on the same Redis Cluster slot.

## Implementation Plan

### Phase 1: Configuration Support
- [ ] Add Redis Cluster configuration option
- [ ] Add cluster-aware key naming toggle
- [ ] Support both legacy and cluster-compatible modes

### Phase 2: Key Naming Strategy
- [ ] Modify key generation functions to use hash tags when cluster mode is enabled
- [ ] Ensure backward compatibility with existing deployments
- [ ] Update all key generation methods in RedisCache

### Phase 3: Integration Testing
- [ ] Create Redis Cluster docker-compose setup
- [ ] Add integration tests for cluster functionality
- [ ] Validate all Lua scripts work with clustered Redis
- [ ] Test migration scenarios

### Phase 4: Documentation & Deployment
- [ ] Update deployment documentation
- [ ] Add Redis Cluster configuration examples
- [ ] Document migration path from single-node to cluster

## Backward Compatibility

The implementation will:
1. Default to legacy key naming for existing deployments
2. Require explicit configuration to enable cluster-compatible mode  
3. Provide migration tools/documentation for upgrading existing deployments
4. Maintain API compatibility

## Testing Strategy

1. **Unit Tests**: Validate key naming functions
2. **Integration Tests**: Test against both single-node and cluster Redis
3. **Migration Tests**: Verify smooth transition scenarios
4. **Performance Tests**: Ensure no degradation in performance

## Risks & Mitigations

### Risk: Breaking existing deployments
**Mitigation**: Feature flag and backward compatibility mode

### Risk: Performance impact from hash tags
**Mitigation**: Benchmark testing and optional enablement

### Risk: Complex migration path
**Mitigation**: Clear documentation and migration tools

## Configuration Examples

### Single Node (Legacy - Default)
```yaml
cache:
  type: REDIS
  redis_cluster_mode: false
redis:
  address: localhost
  port: 6379
```

### Redis Cluster
```yaml  
cache:
  type: REDIS
  redis_cluster_mode: true
  redis_cluster_addresses:
    - redis-node-1:6379
    - redis-node-2:6379  
    - redis-node-3:6379
```

## Success Criteria

1. All existing Lua scripts work with Redis Cluster
2. No breaking changes to existing single-node deployments
3. Comprehensive test coverage for both modes
4. Clear documentation for cluster deployment
5. Performance parity with single-node deployments
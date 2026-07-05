# Deckard Helm Chart

This Helm chart deploys Deckard, an application that provides a gRPC API for managing a cyclic priority queue.

Check the [Deckard's github](https://github.com/takenet/deckard) for more information.

> Currently, we are in the early stages of using our Helm chart, please share any problems you may encounter with it.

## Prerequisites

- Kubernetes 1.24+ (gRPC probes requires Kubernetes 1.24+)
- Helm 3+

## Installing the Chart

To install the Deckard chart, use the following commands:

```bash
helm repo add deckard https://takenet.github.io/deckard/
helm install deckard deckard/deckard
```

## Uninstalling the Chart

To uninstall the `deckard` deployment run:

```bash
helm uninstall deckard
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

> **WARNING**
>
> It will also delete the MongoDB and Redis deployments if they were deployed by the chart.

## Persistence

The Deckard chart deploys MongoDB for storage and Redis for the cache by default. By setting `storage.type` to `MONGODB` and `cache.type` to `REDIS` (the default values), the chart will use the deployed MongoDB and Redis instances for storage and caching. If you want to use an existing MongoDB or Redis, you may set either `mongodb.enabled` or `redis.enabled` to `false`. In this case you must provide existing Kubernetes Secrets for the storage and/or cache connection strings. The chart does not expose direct `storage.uri` or `cache.uri` value overrides.

When `mongodb.enabled` and/or `redis.enabled` are `true`, the chart computes internal service hostnames automatically from the subchart names. If you customize `mongodb.fullnameOverride` or `redis.fullnameOverride`, the generated URIs will follow those names.

For more information about Deckard's configuration, refer to the [Deckard's configuration documentation](https://github.com/takenet/deckard#configuration).

## Housekeeper

Deckard has a housekeeper task that runs periodically with several tasks, such as cleaning up expired items and generating metrics.

All the housekeeper tasks can be run in the same pod as the Deckard gRPC service or in a separate deployment. By default, it is enabled as a separate deployment. If you want to run the housekeeper task in the same pod, set `housekeeper.self.enabled` to `true` and `housekeeper.enabled` to `false`. The housekeeper deployment can be customized with the available configuration options.

## Configuration

The following table lists the configurable parameters of the Deckard chart and their default values.

Check the [values.yaml](https://github.com/takenet/deckard/blob/main/helm/values.yaml) file for more information.

> Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

### Deckard's main deployment configuration

| Parameter                                    | Description                         | Default          |
| -------------------------------------------- | ----------------------------------- | ---------------- |
| `replicaCount`                               | Number of replicas                  | `1`              |
| `image.repository`                           | Deckard image repository            | `blipai/deckard` |
| `image.pullPolicy`                           | Image pull policy                   | `IfNotPresent`   |
| `image.tag`                                  | Image tag (overrides appVersion)    | `""`             |
| `imagePullSecrets`                           | Image pull secrets                  | `[]`             |
| `nameOverride`                               | Override the name of the chart      | `""`             |
| `fullnameOverride`                           | Override the full name of the chart | `""`             |
| `labels`                                     | Additional labels                   | `{}`             |
| `audit.enabled`                              | Enable Deckard's audit system       | `false`          |
| `env`                                        | Additional environment variables    | `[]`             |
| `podAnnotations`                             | Additional pod annotations          | `{}`             |
| `podSecurityContext`                         | Pod security context                | `{}`             |
| `securityContext`                            | Container security context          | `{}`             |
| `service.enabled`                            | Enable Kubernetes service           | `true`           |
| `service.type`                               | Kubernetes service type             | `ClusterIP`      |
| `service.port`                               | Kubernetes service port             | `8081`           |
| `ingress.enabled`                            | Enable ingress                      | `false`          |
| `ingress.className`                          | Ingress class name                  | `""`             |
| `ingress.annotations`                        | Ingress annotations                 | `{}`             |
| `ingress.tls`                                | Ingress TLS configuration           | `[]`             |
| `resources`                                  | Resource requests and limits        | `{}`             |
| `autoscaling.enabled`                        | Enable autoscaling                  | `false`          |
| `autoscaling.minReplicas`                    | Minimum number of replicas          | `1`              |
| `autoscaling.maxReplicas`                    | Maximum number of replicas          | `10`             |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization percentage   | `80`             |
| `nodeSelector`                               | Node selector                       | `{}`             |
| `tolerations`                                | Tolerations                         | `[]`             |
| `affinity`                                   | Affinity                            | `{}`             |

### Housekeeper deployment configuration

| Parameter                        | Description                                                         | Default          |
| -------------------------------- | ------------------------------------------------------------------- | ---------------- |
| `housekeeper.self.enabled`       | Enable housekeeper task in the same pod as the Deckard gRPC service | `false`          |
| `housekeeper.enabled`            | Deploy a separate housekeeper deployment                            | `true`           |
| `housekeeper.image.repository`   | Housekeeper image repository                                        | `blipai/deckard` |
| `housekeeper.image.pullPolicy`   | Housekeeper image pull policy                                       | `IfNotPresent`   |
| `housekeeper.image.tag`          | Housekeeper image tag (overrides appVersion)                        | `""`             |
| `housekeeper.replicaCount`       | Number of replicas for the housekeeper deployment                   | `1`              |
| `housekeeper.labels`             | Additional labels for the housekeeper deployment                    | `{}`             |
| `housekeeper.podAnnotations`     | Additional pod annotations for the housekeeper deployment           | `{}`             |
| `housekeeper.podSecurityContext` | Pod security context for the housekeeper deployment                 | `{}`             |
| `housekeeper.securityContext`    | Container security context for the housekeeper deployment           | `{}`             |
| `housekeeper.env`                | Additional environment variables for the housekeeper deployment     | `[]`             |
| `housekeeper.resources`          | Resource requests and limits for the housekeeper deployment         | `{}`             |
| `housekeeper.nodeSelector`       | Node selector for the housekeeper deployment                        | `{}`             |
| `housekeeper.tolerations`        | Tolerations for the housekeeper deployment                          | `[]`             |
| `housekeeper.affinity`           | Affinity for the housekeeper deployment                             | `{}`             |
| `housekeeper.service.enabled`    | Enable Kubernetes service for the housekeeper deployment            | `true`           |
| `housekeeper.service.type`       | Kubernetes service type for the housekeeper deployment              | `ClusterIP`      |
| `housekeeper.service.port`       | Kubernetes service port for the housekeeper deployment              | `8081`           |

### Deckard's storage configuration

| Parameter                                        | Description                                                    | Default               |
| ------------------------------------------------ | -------------------------------------------------------------- | --------------------- |
| `storage.type`                                   | Deckard storage type (MONGODB, MEMORY)                         | `MONGODB`             |
| `storage.mongodb.database`                       | MongoDB database name for Deckard to use                       | `deckard`             |
| `storage.mongodb.collection`                     | MongoDB collection name for Deckard to use                     | `queue`               |
| `storage.mongodb.queue_configuration_collection` | MongoDB queue configuration collection name for Deckard to use | `queue_configuration` |

### Deckard's cache configuration

| Parameter                       | Description                                                                 | Default |
| ------------------------------- | --------------------------------------------------------------------------- | ------- |
| `cache.type`                    | Deckard cache type (REDIS, MEMORY)                                          | `REDIS` |
| `cache.redis.cluster.mode`      | Enable Deckard Redis Cluster mode (`DECKARD_REDIS_CLUSTER_MODE`)            | `false` |

Redis connection details (address, credentials, database, TLS, timeouts, pool size, etc.) are configured through the `cache.connectionSecret` value used as `DECKARD_CACHE_URI`.

- Standalone URI example: `redis://user:pass@redis.example:6379/0`
- Standalone TLS URI example: `rediss://user:pass@redis.example:6380/0`
- Optional insecure TLS mode for development only (standalone only, see [Redis Cluster with Helm](#redis-cluster-with-helm)): `rediss://user:pass@redis.example:6380/0?skip_verify=true`
- Cluster mode (`cache.redis.cluster.mode=true`): the secret value uses go-redis's cluster URL format - a base URI plus repeated `addr=host:port` query parameters for the remaining seed nodes, e.g. `redis://node-1:6379?addr=node-2:6379&addr=node-3:6379`.

### Connection secret configuration

| Parameter                                 | Description                                              | Default       |
| ----------------------------------------- | -------------------------------------------------------- | ------------- |
| `storage.connectionSecret.existingSecret` | Existing Secret containing the storage connection string | `""`          |
| `storage.connectionSecret.key`            | Secret key for the storage connection string             | `storage-uri` |
| `cache.connectionSecret.existingSecret`   | Existing Secret containing the cache connection string   | `""`          |
| `cache.connectionSecret.key`              | Secret key for the cache connection string               | `cache-uri`   |

When `mongodb.enabled=false` or `redis.enabled=false`, use the corresponding `storage.connectionSecret.*` or `cache.connectionSecret.*` settings to provide external connection URIs.

The legacy top-level `connectionSecret.storage.*` and `connectionSecret.cache.*` values are still supported for backward compatibility, but are deprecated. When both legacy and scoped values are set, the scoped `storage.connectionSecret.*` and `cache.connectionSecret.*` values take precedence.

### Redis' Chart configuration

If the Redis is enabled and you want to deploy your own Redis using this chart, you may set redis configurations as above:

For more `redis` configurations check [bitnami's chart available configurations](https://github.com/bitnami/charts/tree/main/bitnami/redis/)

| Parameter             | Description                            | Default      |
| --------------------- | -------------------------------------- | ------------ |
| `redis.enabled`       | Deploy a Redis using the bitnami chart | `true`       |
| `redis.architecture`  | Redis architecture                     | `standalone` |
| `redis.auth.enabled`  | Enable Redis authentication            | `true`       |
| `redis.auth.password` | Redis password for the bitnami/redis subchart (you may also use `redis.auth.existingSecret`) | `""`    |

> If you disabled the redis chart using `redis.enabled` as `false`, any property with the prefix `redis.` will be ignored.

### Redis Cluster with Helm

Deckard's Redis Cluster mode is configured through `cache.redis.cluster.*` values (application-level config), not by toggling only the Redis subchart architecture.

- For external Redis Cluster, set `redis.enabled=false`, provide your cache connection secret, and set:

```yaml
cache:
  type: REDIS
  redis:
    cluster:
      mode: true
```

and set `cache.connectionSecret.existingSecret` (with `cache.connectionSecret.key`) to a Secret whose value is a cluster URL in go-redis's format: a base URI plus repeated `addr=host:port` query parameters, e.g. `redis://redis-node-1:6379?addr=redis-node-2:6379&addr=redis-node-3:6379`. Redis Cluster has no database selector (no `SELECT`), and its URL parser does not accept `skip_verify`.

- The built-in `bitnami/redis` dependency in this chart is documented and validated for standalone cache usage only; enabling it together with `cache.redis.cluster.mode=true` fails chart validation.

### MongoDB's Chart configuration

If the MongoDB is enabled and you want to deploy your own MongoDB using this chart, you may set MongoDB configurations as above:

For more `mongodb` configurations check [bitnami's chart available configurations](https://github.com/bitnami/charts/tree/main/bitnami/mongodb/)

| Parameter                   | Description                               | Default      |
| --------------------------- | ----------------------------------------- | ------------ |
| `mongodb.enabled`           | Deploy a MongoDB using the bitnami chart. | `true`       |
| `mongodb.auth.enabled`      | Enable MongoDB authentication             | `true`       |
| `mongodb.auth.rootUser`     | MongoDB root username                     | `deckard`    |
| `mongodb.auth.rootPassword` | MongoDB root password for the bitnami/mongodb subchart (you may also use `mongodb.auth.existingSecret`) | `""`    |
| `mongodb.architecture`      | MongoDB architectur                       | `standalone` |

> If you disabled the MongoDB chart using `mongodb.enabled` as `false`, any property with the prefix `mongodb.` will be ignored.

## Contributing

Please refer to the [contribution guidelines](https://github.com/takenet/deckard/blob/main/CONTRIBUTING.md) for more information on how to get involved.

## License

This project is licensed under the [MIT License](https://github.com/takenet/deckard/blob/main/LICENSE).

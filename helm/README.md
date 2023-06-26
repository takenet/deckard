# Deckard Helm Chart

This Helm chart deploys Deckard, an application that provides a gRPC API for managing a cyclic priority queue.

Check the [Deckard's github](https://github.com/takenet/deckard) for more information.

> Currently, we are in the early stages of using our Helm chart, please share any problems you may encounter with it.

## Prerequisites

- Kubernetes 1.16+
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

The Deckard chart deploys MongoDB for storage and Redis for the cache by default. By setting `storage.type` to `MONGODB` and `cache.type` to `REDIS`, the chart will use the deployed MongoDB and Redis instances for storage and caching. If you want to use existing MongoDB or self-deployed Redis, set `mongodb.enabled` and `redis.enabled` to `false`, respectively. You can also modify the storage and cache URIs manually if needed.

For more information about Deckard's configuration, refer to the [Deckard's configuration documentation](https://github.com/takenet/deckard#configuration).

## Housekeeper

Deckard has a housekeeper task that runs periodically with several tasks, such as cleaning up expired items and generating metrics.

All the housekeeper tasks can be run in the same pod as the Deckard gRPC service or in a separate deployment. By default, it is enabled as a separate deployment. If you want to run the housekeeper task in the same pod, set `housekeeper.self.enabled` to `true` and `housekeeper.enabled` to `false`. The housekeeper deployment can be customized with the available configuration options.

## Configuration

The following table lists the configurable parameters of the Deckard chart and their default values.

Check the [values.yaml](https://github.com/takenet/deckard/blob/main/helm/values.yaml) file for more information.

> Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

### Deckard's main deployment configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `replicaCount` | Number of replicas | `1` |
| `image.repository` | Deckard image repository | `blipai/deckard` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.tag` | Image tag (overrides appVersion) | `""` |
| `imagePullSecrets` | Image pull secrets | `[]` |
| `nameOverride` | Override the name of the chart | `""` |
| `fullnameOverride` | Override the full name of the chart | `""` |
| `labels` | Additional labels | `{}` |
| `audit.enabled` | Enable Deckard's audit system | `false` |
| `env` | Additional environment variables | `[]` |
| `podAnnotations` | Additional pod annotations | `{}` |
| `podSecurityContext` | Pod security context | `{}` |
| `securityContext` | Container security context | `{}` |
| `service.enabled` | Enable Kubernetes service | `true` |
| `service.type` | Kubernetes service type | `ClusterIP` |
| `service.port` | Kubernetes service port | `8081` |
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.annotations` | Ingress annotations | `{}` |
| `ingress.tls` | Ingress TLS configuration | `[]` |
| `resources` | Resource requests and limits | `{}` |
| `autoscaling.enabled` | Enable autoscaling | `false` |
| `autoscaling.minReplicas` | Minimum number of replicas | `1` |
| `autoscaling.maxReplicas` | Maximum number of replicas | `10` |
| `autoscaling.targetCPUUtilizationPercentage` | Target CPU utilization percentage | `80` |
| `nodeSelector` | Node selector | `{}` |
| `tolerations` | Tolerations | `[]` |
| `affinity` | Affinity | `{}` |

### Housekeeper deployment configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `housekeeper.self.enabled` | Enable housekeeper task in the same pod as the Deckard gRPC service | `false` |
| `housekeeper.enabled` | Deploy a separate housekeeper deployment | `true` |
| `housekeeper.image.repository` | Housekeeper image repository | `blipai/deckard` |
| `housekeeper.image.pullPolicy` | Housekeeper image pull policy | `IfNotPresent` |
| `housekeeper.image.tag` | Housekeeper image tag (overrides appVersion) | `""` |
| `housekeeper.replicaCount` | Number of replicas for the housekeeper deployment | `1` |
| `housekeeper.labels` | Additional labels for the housekeeper deployment | `{}` |
| `housekeeper.podAnnotations` | Additional pod annotations for the housekeeper deployment | `{}` |
| `housekeeper.podSecurityContext` | Pod security context for the housekeeper deployment | `{}` |
| `housekeeper.securityContext` | Container security context for the housekeeper deployment | `{}` |
| `housekeeper.env` | Additional environment variables for the housekeeper deployment | `[]` |
| `housekeeper.resources` | Resource requests and limits for the housekeeper deployment | `{}` |
| `housekeeper.nodeSelector` | Node selector for the housekeeper deployment | `{}` |
| `housekeeper.tolerations` | Tolerations for the housekeeper deployment | `[]` |
| `housekeeper.affinity` | Affinity for the housekeeper deployment | `{}` |
| `housekeeper.service.enabled` | Enable Kubernetes service for the housekeeper deployment | `true` |
| `housekeeper.service.type` | Kubernetes service type for the housekeeper deployment | `ClusterIP` |
| `housekeeper.service.port` | Kubernetes service port for the housekeeper deployment | `8081` |
| `housekeeper.storage.uri` | Housekeeper storage URI | `""` |
| `housekeeper.cache.uri` | Housekeeper cache URI | `""` |

### Deckard's storage configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `storage.type` | Deckard storage type (MONGODB, MEMORY) | `MONGODB` |
| `storage.uri` | Deckard storage URI | `""` |
| `storage.mongodb.database` | MongoDB database name for Deckard to use | `deckard` |
| `storage.mongodb.collection` | MongoDB collection name for Deckard to use | `queue` |
| `storage.mongodb.queue_configuration_collection` | MongoDB queue configuration collection name for Deckard to use | `queue_configuration` |

### Deckard's cache configuration

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `cache.type` | Deckard cache type (REDIS, MEMORY) | `REDIS` |
| `cache.uri` | Deckard cache URI | `""` |
| `cache.redis.database` | Redis database for Deckard to use | `0` |

### Redis' Chart configuration

The following table lists the default configuration values for the Redis chart.

For more `redis` configurations check [bitnami's chart available configurations](https://github.com/bitnami/charts/tree/main/bitnami/redis/)

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `redis.enabled` | Deploy a Redis using the bitnami chart | `true` |
| `redis.architecture` | Redis architecture | `standalone` |
| `redis.auth.enabled` | Enable Redis authentication | `true` |
| `redis.auth.password` | Redis password | `deckard` |

### MongoDB's Chart configuration

The following table lists the default configuration values for the MongoDB chart.

For more `mongodb` configurations check [bitnami's chart available configurations](https://github.com/bitnami/charts/tree/main/bitnami/mongodb/)

| Parameter | Description | Default |
| --------- | ----------- | ------- |
| `mongodb.enabled` | Deploy a MongoDB using the bitnami chart. | `true` |
| `mongodb.auth.enabled` | Enable MongoDB authentication | `true` |
| `mongodb.auth.rootUser` | MongoDB root username | `deckard` |
| `mongodb.auth.rootPassword` | MongoDB root password | `deckard` |
| `mongodb.architecture` | MongoDB architectur | `standalone` |

## Contributing

Please refer to the [contribution guidelines](https://github.com/takenet/deckard/blob/main/CONTRIBUTING.md) for more information on how to get involved.

## License

This project is licensed under the [MIT License](https://github.com/takenet/deckard/blob/main/LICENSE).
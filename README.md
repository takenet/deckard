# Deckard: A Highly Scalable Cyclic Priority Queue

[![License](https://img.shields.io/github/license/takenet/deckard)](https://github.com/takenet/deckard/blob/main/LICENSE) [![codecov](https://codecov.io/gh/takenet/deckard/branch/main/graph/badge.svg?token=IMT8NWZ69A)](https://codecov.io/gh/takenet/deckard) [![Go Report Card](https://goreportcard.com/badge/github.com/takenet/deckard)](https://goreportcard.com/report/github.com/takenet/deckard)

[![Artifact Hub](https://img.shields.io/badge/helm-Artifact_Hub_%28Deckard%29-blue?logo=helm&link=https://artifacthub.io/packages/helm/deckard/deckard)](https://artifacthub.io/packages/helm/deckard/deckard) [![Docker](https://img.shields.io/docker/image-size/blipai/deckard/latest?logo=docker)](https://hub.docker.com/r/blipai/deckard)

[![Golang](https://img.shields.io/github/go-mod/go-version/takenet/deckard?logo=go)](https://pkg.go.dev/github.com/takenet/deckard) [![Maven Central](https://img.shields.io/maven-central/v/ai.blip/deckard)](https://central.sonatype.com/artifact/ai.blip/deckard/) [![Nuget org](https://img.shields.io/nuget/v/deckard?logo=nuget)](https://www.nuget.org/packages/Deckard)

Deckard is a priority queue system inspired by projects such as Google Cloud PubSub, Nats, Kafka, and others. Its main distinction lies in its ability to associate a priority score with each message and have a queue that can be optionally cyclic. This means that messages can be delivered again after a user-managed time, allowing throughput configuration and many other features, all built using GoLang and gRPC to be as scalable as possible.

![deckard](docs/deckard_cartoon.webp)

Briefly:
- An application inserts a message to be queued and its configuration (TTL, metadata, payload, priority score, etc).
    - The message will be prioritized with a default timestamp-based algorithm if the provided score is 0 (the default value).
- A worker application pull messages from Deckard at regular intervals and performs any processing.
    - The worker can also send a score filter (max score and min score) in the pull request. This enables theoughput controlling with score algorithms and many other features.
    - When it finishes processing a message, the application must notify with the processing result.
    - When notifying, the application may also provide a lock time, to lock the message for a certain duration of time before being requeued and delivered again. Locking mechanism have a 1-second precision.
    - It is also possible to notify a message without lock but changing its priority score to be delivered again.
- When the message's TTL is reached, it stops being delivered;
    - For some use cases the TTL can be set as infinite.
    - An application can also remove the message when notifying.

## Motivation

Deckard was born from an initiative to simplify and unify applications called Frontiers which were part of STILINGUE's orchestration system for data gathering.

Several different implementations were made, but they proved to have limitations such as:
- Debugging
- Genericity
- Scalability
- Auditability
- Observability
- Prioritization
- Developer friendliness

Deckard was created to solve these problems and to be used in any application that needs to queue data and prioritize it.

The **main** objectives of the project are:
- *Generic*: any data and application should be able to use Deckard to queue messages;
- *Observable*: it should be easy to visualize what happened in a request and easy to investigate problems with audit;
- *Developer friendly*: easy to understand and pleasant to use;
- *Scalable*: it should be possible to support thousands of requests per second and millions of messages;

### **What Deckard is not?**

It is important to note that Deckard is not a conventional messaging/queue system. If your use case does not involve priority, cyclic queuing, or a locking mechanism, it is recommended to consider alternatives such as GCP PubSub, Kafka, RabbitMQ, Azure Service Bus, Amazon SQS, or other messaging systems.

## Project Status

Deckard has been used in a production environment since 2019, handling billions of messages and thousands of requests per second. Since its initial internal release, Deckard has undergone significant improvements and enhancements.

The performance and reliability of Deckard are directly dependent on the storage and cache services. Therefore, it is crucial to configure MongoDB and Redis correctly to ensure optimal performance. Redis is responsible for storing all priority queues and has a significant impact on Deckard's performance, while MongoDB usage is simpler and has a lower impact on performance.

> Currently, we have projects that utilize MongoDB in both a virtual machine environment and a Kubernetes environment. However, it's worth noting that we have had more extensive usage of MongoDB in a virtual machine (VM) environment compared to Kubernetes. When deploying Deckard, it is important to configure MongoDB properly, following MongoDB's recommendations for production environments (documented here: [MongoDB Production Notes](https://www.mongodb.com/docs/manual/administration/production-notes/)).

While the project is being released with a `0.0.x` version due to necessary code modifications for open-sourcing, it has been thoroughly tested and proven to be reliable in production scenarios.

Please refer to our [issues](https://github.com/takenet/deckard/issues) section for more details. Your feedback and suggestions are highly appreciated and can be shared in our [discussions](https://github.com/takenet/deckard/discussions).

## Getting Started

To quickly get started with Deckard, please consult our [getting started guide](/docs/getting-started.md), which provides step-by-step instructions. Additionally, we have provided [client documentation](/docs/using.md) for various programming languages to assist you in integrating Deckard into your projects.

You can also access our documentation on Deckard's components [here](docs/components.md). More details on the project structure can be found [here](CONTRIBUTING.md).

### Running Deckard

Here's a quick guide on how to run Deckard. You should check the [getting started guide](/docs/getting-started.md) for more details.

On `Linux` you can run it from sources with:
```shell
make run
```

You can run it with Docker:
```shell
docker run --rm -p 8081:8081 blipai/deckard
```

> By default for Docker and Linux it will use a memory storage and a memory cache engine.
>
> To change the default configuration see the [configuration section](/README.md?#configuration).

You can also run it in a Kubernetes cluster using Helm:

> It will deploy a MongoDB for storage and a Redis for cache.
>
> Check the chart [values.yaml](helm/values.yaml) to see all available configurations.

```shell
helm repo add deckard https://takenet.github.io/deckard/
helm install deckard deckard/deckard
```

You may also download the latest release from the [releases](https://github.com/takenet/deckard/releases) page and execute it.

## Configuration

The current implementation delegates the configuration to environment variables.

All available environment variables are listed below:

### Overall Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_DEBUG` | `false` | To enable debug mode to log more information. |
| `DECKARD_LOG_TYPE` | `json` | The log type to use. Available: json, text |
| `DECKARD_GRPC_ENABLED` | `true` | To enable the gRPC service. You can disable gRPC service if you want an instance to perform only housekeeper tasks. |
| `DECKARD_GRPC_PORT` | `8081` | The gRPC port to listen. |

### gRPC Server Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_GRPC_SERVER_KEEPALIVE_TIME` |  | The interval after which a keepalive ping is sent. |
| `DECKARD_GRPC_SERVER_KEEPALIVE_TIMEOUT` |  | The duration the gRPC server waits for a keepalive response before closing the connection. |
| `DECKARD_GRPC_SERVER_MAX_CONNECTION_IDLE` |  | The maximum duration a connection can remain idle. |
| `DECKARD_GRPC_SERVER_MAX_CONNECTION_AGE` |  | The maximum duration a connection can exist. |
| `DECKARD_GRPC_SERVER_MAX_CONNECTION_AGE_GRACE` |  | The additional time the gRPC server allows for a connection to complete its current operations before closing it after reaching the maximum connection age. |
| `DECKARD_GRPC_SERVER_MAX_RECV_MSG_SIZE` | 4194304 | The maximum message size that can be received. |
| `DECKARD_GRPC_SERVER_MAX_SEND_MSG_SIZE` | 4194304 | The maximum message size that can be sent. |

> Values should be specified using time units such as `1s` for seconds, `1m` for minutes, `1h` for hours.

The default values depends on the server implementation. For more information check these links:
- [Keepalive configuration specification](https://grpc.io/docs/guides/keepalive/#keepalive-configuration-specification)
- [`grpc-go/internal/transport/defaults.go`](https://github.com/grpc/grpc-go/blob/master/internal/transport/defaults.go)

### Cache Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_CACHE_TYPE` | `MEMORY` | The cache implementation to use. Available: MEMORY, REDIS |
| `DECKARD_CACHE_URI` | | The cache Connection URI to connect with the cache service. Currently only a Redis URI is accepted. It will take precedence over any other environment variable related to the connection Redis connection. |
| `DECKARD_REDIS_ADDRESS` | `localhost` | The redis address to connect while using redis cache implementation. It will be overriden by `DECKARD_CACHE_URI` if present.  |
| `DECKARD_REDIS_PASSWORD` |  | The redis password to connect while using redis cache implementation. It will be overriden by `DECKARD_CACHE_URI` if present. |
| `DECKARD_REDIS_PORT` | `6379` | The redis port to connect while using redis cache implementation. It will be overriden by `DECKARD_CACHE_URI` if present. |
| `DECKARD_REDIS_DB` | `0` | The database to use while using redis cache implementation. It will be overriden by `DECKARD_CACHE_URI` if present. |

### Storage Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_STORAGE_TYPE` | `MEMORY` | The storage implementation to use. Available: MEMORY, MONGODB |
| `DECKARD_STORAGE_URI` |  | The storage Connection URI to connect with the storage service. Currently only a MongoDB URI is accepted. It can override any other environment variable related to the connection MongoDB connection since it takes precedence. |
| `DECKARD_MONGODB_ADDRESSES` | `localhost:27017` | The MongoDB addresses separated by comma to connect while using MongoDB storage implementation. It can be overridden by `DECKARD_STORAGE_URI`. |
| `DECKARD_MONGODB_AUTH_DB` |  | The MongoDB auth database to authenticate while using MongoDB storage implementation. It can be overridden by `DECKARD_STORAGE_URI`. |
| `DECKARD_MONGODB_PASSWORD` |  | The MongoDB password to authenticate while using MongoDB storage implementation. It can be overridden by `DECKARD_STORAGE_URI`. |
| `DECKARD_MONGODB_DATABASE` | `deckard` | The MongoDB database to use to store messages while using MongoDB storage implementation. |
| `DECKARD_MONGODB_COLLECTION` | `queue` | The MongoDB collection to use to store messages while using MongoDB storage implementation. |
| `DECKARD_MONGODB_USER` |  | The MongoDB user to authenticate while using MongoDB storage implementation. It can be overridden by `DECKARD_STORAGE_URI`. |
| `DECKARD_MONGODB_SSL` | `false` | To enable SSL while using MongoDB storage implementation. It can be overridden by `DECKARD_STORAGE_URI`. |
| `DECKARD_MONGODB_QUEUE_CONFIGURATION_COLLECTION` | `queue_configuration` | The MongoDB collection to use to store queue configurations while using MongoDB storage implementation. |

### Housekeeper Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_HOUSEKEEPER_ENABLED` | `true` | To enable housekeeper tasks. |
| `DECKARD_HOUSEKEEPER_TASK_TIMEOUT_DELAY` | `1s` | The delay between each timeout task execution. |
| `DECKARD_HOUSEKEEPER_TASK_UNLOCK_DELAY` | `1s` | The delay between each unlock task execution. |
| `DECKARD_HOUSEKEEPER_TASK_UPDATE_DELAY` | `1s` | The delay between each update task execution. |
| `DECKARD_HOUSEKEEPER_TASK_TTL_DELAY` | `1s` | The delay between each ttl task execution. |
| `DECKARD_HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY` | `1s` | The delay between each max elements task execution. |
| `DECKARD_HOUSEKEEPER_TASK_METRICS_DELAY` | `60s` | The delay between each metrics task execution. |

### Audit Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_AUDIT_ENABLED` | `false` | To enable auditing. |
| `DECKARD_ELASTIC_ADDRESS` | `http://localhost:9200/` | A ElasticSearch address to connect to store audit information.  |
| `DECKARD_ELASTIC_PASSWORD` |  | A ElasticSearch password to connect to store audit information.  |
| `DECKARD_ELASTIC_USER` |  | A ElasticSearch user to connect to store audit information.  |

### Metrics Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_METRICS_ENABLED` | `true` | To enable exporting metrics to the metrics endpoint. |
| `DECKARD_METRICS_PORT` | `22022` | The metrics http port to listn. |
| `DECKARD_METRICS_PATH` | `/metrics` | The metrics http path to expose metrics. |
| `DECKARD_METRICS_HISTOGRAM_BUCKETS` | `0,1,2,5,10,15,20,30,35,50,100,200,400,600,800,1000,1500,2000,5000,10000` | The histogram buckets to use to expose metrics. |
| `DECKARD_METRICS_OPENMETRICS_ENABLED` | `true` | If true, the OpenMetrics encoding is added to the possible options during content negotiation. |

### TLS Configuration

To learn more about gRPC TLS configuration please refer to [gRPC Auth](https://grpc.io/docs/guides/auth/).

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_TLS_CLIENT_AUTH_TYPE` |  `NoClientCert` | The type of client authentication TLS verification to use. Available: `NoClientCert`, `RequestClientCert`, `RequireAnyClientCert`, `VerifyClientCertIfGiven`, `RequireAndVerifyClientCert`.  |
| `DECKARD_TLS_SERVER_CERT_FILE_PATHS` |  | A comma-delimited list of absolute file paths to PEM-encoded certificates.  |
| `DECKARD_TLS_SERVER_KEY_FILE_PATHS` |  | A comma-delimited list of absolute file paths to PEM-encoded private keys.  |
| `DECKARD_TLS_CLIENT_CERT_FILE_PATHS` |  | A comma-delimited list of absolute file paths to PEM-encoded certificates to enable mutual TLS.  |

## Contributing

We are always looking for new contributors to help us improve Deckard.

If you want to contribute to Deckard, please read our [contributing guide](CONTRIBUTING.md) which includes how to build, run and test Deckard and a complete description of our project structure.

## License

Deckard is licensed under the [MIT License](LICENSE).

## Acknowledgments

We would like to thank the following people for their initial contributions building Deckard's first version:
- Lucas Soares: [@lucasoares](https://github.com/lucasoares)
- Gustavo Paiva: [@paivagustavo](https://github.com/paivagustavo)
- Cézar Augusto: [@cezar-tech](https://github.com/cezar-tech)
- Júnior Rhis: [@juniorrhis](https://github.com/juniorrhis)

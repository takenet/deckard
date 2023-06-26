# Deckard: A Highly Scalable Cyclic Priority Queue

[![codecov](https://codecov.io/gh/takenet/deckard/branch/main/graph/badge.svg?token=IMT8NWZ69A)](https://codecov.io/gh/takenet/deckard)

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/deckard)](https://artifacthub.io/packages/search?repo=deckard)

Deckard is a priority queue system inspired by projects like: Google Cloud PubSub, Nats, Kafka and others.

![deckard](docs/deckard_cartoon.webp)

The main difference is that Deckard has a priority associated with each message and it is optionally cyclic, meaning that the message can be delivered again after a certain user-managed time.

Briefly:
- An application inserts a message to be queued and its configuration (TTL, metadata, payload, etc).
    - The message will be prioritized with a default timestamp-based algorithm. The priority can also be provided by the application.
- A worker application pull messages from Deckard at regular intervals and performs any processing.
    - When it finishes processing a message, the application must notify with the processing result.
    - When notifying, the application may provide a lock time, to lock the message for a certain duration of time before being requeued and delivered again.
    - It is also possible to notify a message changing its priority.
- When the message's TTL is reached, it stops being delivered;
    - For some use cases the TTL can be set as infinite.
    - An application can also remove the message when notifying.

## Motivation

Deckard was born from an initiative to simplify and unify applications called Frontiers which were part of STILINGUE's orchestration system for data gathering.

Several different implementations were made, but they proved to have several limitations such as:
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

It is not a normal messaging/queue system. If you don't have a use case that needs priority and cyclic queuing or locking mechanism, you should use GCP PubSub, Kafka, RabbitMQ, Azure Service Bus, Amazon SQS, or any other messaging system.

## Project Status

Deckard has been used in a production environment for over 2 years handling millions of messages and thousands of requests per second.

To be able to open source the project we had to make some changes to the code and that is the reason we opted to release it with a `0.0.x` version.

We also know few issues we need to work but currently we are very confident that it can be used in production environments. Check our [issues](https://github.com/takenet/deckard/issues) to see what we are working on.

Please let us know if you find any issues or have any suggestions in our [discussions](https://github.com/takenet/deckard/discussions).

## Getting Started

A [getting started guide](/docs/getting-started.md) is available to help you to start using Deckard.

Check also the [client documentation](/docs/using.md) to see how to use Deckard in a project using your favorite language.

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

We currently use the [viper](github.com/spf13/viper) project to manage configurations and the current implementation delegates the configuration to environment variables.

All available environment variables are listed below:

### Overall Configuration

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| `DECKARD_DEBUG` | `false` | To enable debug mode to log more information. |
| `DECKARD_LOG_TYPE` | `json` | The log type to use. Available: json, text |
| `DECKARD_GRPC_ENABLED` | `true` | To enable the gRPC service. You can disable gRPC service if you want an instance to perform only housekeeper tasks. |
| `DECKARD_GRPC_PORT` | `8081` | The gRPC port to listen. |

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

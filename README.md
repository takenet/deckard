# Deckard - A Cyclic Priority Queue (CPQ)

Deckard is a messaging system inspired by projects like: Google Cloud PubSub, Nats, Kafka and others.

![deckard](docs/deckard_cartoon.webp)

The main difference is that Deckard has a priority associated with each message and it is optionally cyclic, meaning that the message can be delivered again after a certain user-managed time.

Briefly:
- An application inserts a data to be queued and its configuration (TTL, priority, etc.);
- A second application fetches data from the deckard at regular intervals and performs any processing;
    - When it finishes processing a data, this application notifies the deckard with the processing result and its new priority.
    - The application may also send blocking time to the deckard, meaning that the message will not be delivered until the blocking time is reached.
    - It is also possible to send a message to the deckard with a new priority, meaning that the message will be prioritized and then delivered.
- When the message's time limit is reached or an application removes it, it stops being delivered;

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

## What Deckard is not?

It is not a project that has business logic. No logic of any product should be implemented inside Deckard. It was made to be generic and customizable for each individual use case.

## Build

To build the Deckard service you must have [golang](https://golang.org/dl/) installed. You also must have grpc and protobuf compilers for golang installed:
```shell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

Clone the project:
```shell
git clone git@github.com:takenet/deckard.git
```

Generate files for the gRPC service and apis with the following command:
```shell
make gen-proto
```

Build a Deckard executable:
```shell
make
```

The executable will be generated in the `exec` folder.

## Running Deckard

If you built the Deckard executable you can run it directly (`.exe` for Windows):
```shell
./exec/deckard
```

You can also run it directly with the following command:
```shell
make run
```

Running Deckard with Docker:
```shell
docker run --rm -p 8081:8081 blipai/deckard
```

You may also download the latest release from the [releases](https://github.com/takenet/deckard/releases) page and execute it.

> By default it will use a memory storage and a memory cache engine.
>
> To change the default configuration see the [configuration section](#configuration).

## Running tests

To run project tests you must first generate all mock files with the following command:
```shell
make gen-mocks
```

> You also need to have [mockgen](https://github.com/golang/mock) installed.
>
> Considerations:
> - Any modification in any interface must be followed by the generation of the mock files.
>
> - Any modification in the `.proto` file must be followed by the generation of the source files using `make gen-proto`.

Running all unit and integration tests
```shell
make test
```

Running only unit tests
```shell
make unit-test
```

Running only integration tests
```shell
make integration-test
```

We are currently using the [testify](https://github.com/stretchr/testify) package.

### Integration tests

To run the integration tests you must have the following services available in `localhost`:

- Storage:
    - MongoDB: `localhost:27017`
- Cache:
    - Redis: `localhost:6379`

Unit tests and integration tests may be found in the same file, but all integration tests must use the [short](https://golang.org/pkg/testing/#Short) flag.

Every integration tests have the suffix `Integration` in their name.

### Mocks

We use the [GoMock](https://github.com/golang/mock) project to generate mocks.

To update the mocks you must run the following command:
```shell
make gen-mocks
```

Mocks are generated in the [/internal/mocks](/internal/mocks) folder.

When creating interfaces with the need to generate mocks, you must add the following directive to the interface file:
```go
//go:generate mockgen -destination=<path_to_mocks>/mock_<file>.go -package=mocks -source=<file>.go
```

## Docker Image

To generate the Deckard image we use the [ko](https://github.com/google/ko) tool.

>**ko** is a tool for building images and binaries for Go applications.
>
>It is a simple, fast container image builder for Go applications.
>
> To use it you may need to set your GOROOT environment variable (https://github.com/ko-build/ko/issues/218):
>
>```shell
>export GOROOT="$(go env GOROOT)"
>```
> You may need to set a different GOROOT depending on your environment, please deal with it carefully.

To generate a local image just run the following command:
```shell
make build-local-image
```

And then run it:
```shell
docker run --rm -p 8081:8081 ko.local/deckard:<build_version>
```

> Change the `build_version` with the version logged while building the image.

## Java

We currently provide a Java API for the Deckard service. To build java source files you must have [Maven](https://maven.apache.org/) installed.

To generate the Java API files you must run the following command:
```shell
make gen-java
```

## Organization and Components

The project has as its base a RPC service with the Google implementation known as [gRPC](https://github.com/grpc) using [Protocol Buffers](https://developers.google.com/protocol-buffers).

It is organized in the following folders:

```
deckard
├── dashboards              # Dashboards templates for Grafana (metrics) and Kibana (audit)
├── docker                  # Docker compose to help running integration tests and base docker image file
├── docs                    # Documentation files
├── internal                # Internal .go files
│   ├── audit               # Audit system
│   ├── cmd                 # Executable main file for the Deckard service
│   ├── config              # Configuration variables managed by viper
│   ├── logger              # Logging configuration
│   ├── messagepool         # Contains all main implementation files and housekeeping program logic
│   │   ├── cache           # Caching implementation
│   │   ├── entities        # Message and QueueConfiguration internal definitions
│   │   ├── queue           # Queue definition
│   │   ├── storage         # Storage implementation
│   │   └── utils           # Utility package for data conversion
│   ├── metrics             # Package with metrics definitions
│   ├── mocks               # Auto-generated files via mockgen (created after running `make gen-mocks`)
│   ├── project             # Project package with information used by the version control system
│   ├── service             # Internal service implementation of the gRPC service
│   ├── shutdown            # Shutdown package with graceful shutdown implementation
│   └── trace               # Trace package with tracing implementation
├── java                    # Files for generating the Java API
├── proto                   # .proto files with the all definitions
└── service
    └── api                 # Folder with generated API files via protobuf for golang
```

> See the [configurations section](#configuration) to see how to configure any internal component.

For more details about each component see the [components documentation](docs/components.md).

## Configuration

We currently use the [viper](github.com/spf13/viper) project to manage configurations and the current implementation delegates the configuration to environment variables.

The following table shows all available configurations:

| Environment Variable         | Default | Description |
|------------------------------|---------|-------------|
| DECKARD_DEBUG | false | To enable debug mode to log more information. |
| DECKARD_LOG_TYPE | json | The log type to use. Available: json, text |
| DECKARD_STORAGE_TYPE | MEMORY | The storage implementation to use. Available: MEMORY, MONGODB |
| DECKARD_CACHE_TYPE | MEMORY | The cache implementation to use. Available: MEMORY, REDIS |
| DECKARD_HOUSEKEEPER_ENABLED | true | To enable housekeeper tasks. |
| DECKARD_GRPC_ENABLED | true | To enable the gRPC service. |
| DECKARD_GRPC_PORT | 8081 | The gRPC port to listen. |
| DECKARD_REDIS_ADDRESS | localhost | The redis address to connect while using redis cache implementation.  |
| DECKARD_REDIS_PASSWORD |  | The redis password to connect while using redis cache implementation. |
| DECKARD_REDIS_PORT | 6379 | The redis port to connect while using redis cache implementation.  |
| DECKARD_REDIS_DB | 0 | The database to use while using redis cache implementation.  |
| DECKARD_AUDIT_ENABLED | false | To enable auditing. |
| DECKARD_ELASTIC_ADDRESS | http://localhost:9200/ | A ElasticSearch address to connect to store audit information.  |
| DECKARD_ELASTIC_PASSWORD |  | A ElasticSearch password to connect to store audit information.  |
| DECKARD_ELASTIC_USER |  | A ElasticSearch user to connect to store audit information.  |
| DECKARD_MONGO_ADDRESSES | localhost:27017 | The MongoDB address to connect while using MongoDB storage implementation. |
| DECKARD_MONGO_AUTH_DB |  | The MongoDB auth database to authenticate while using MongoDB storage implementation. |
| DECKARD_MONGO_PASSWORD |  | The MongoDB password to authenticate while using MongoDB storage implementation. |
| DECKARD_MONGO_DATABASE | deckard | The MongoDB database to use to store messages while using MongoDB storage implementation. |
| DECKARD_MONGO_COLLECTION | queue | The MongoDB collection to use to store messages while using MongoDB storage implementation. |
| DECKARD_MONGO_USER |  | The MongoDB user to authenticate while using MongoDB storage implementation. |
| DECKARD_MONGO_SSL | false | To enable SSL while using MongoDB storage implementation. |
| DECKARD_MONGO_QUEUE_CONFIGURATION_COLLECTION | queue_configuration | The MongoDB collection to use to store queue configurations while using MongoDB storage implementation. |
| DECKARD_HOUSEKEEPER_TASK_TIMEOUT_DELAY | 1s" | The delay between each timeout task execution. |
| DECKARD_HOUSEKEEPER_TASK_UNLOCK_DELAY | 1s" | The delay between each unlock task execution. |
| DECKARD_HOUSEKEEPER_TASK_UPDATE_DELAY | 1s" | The delay between each update task execution. |
| DECKARD_HOUSEKEEPER_TASK_TTL_DELAY | 1s" | The delay between each ttl task execution. |
| DECKARD_HOUSEKEEPER_TASK_MAX_ELEMENTS_DELAY | 1s" | The delay between each max elements task execution. |
| DECKARD_HOUSEKEEPER_TASK_METRICS_DELAY | 60s" | The delay between each metrics task execution. |

## Contributing

We are always looking for new contributors to help us improve Deckard.

If you want to contribute to Deckard, please read our [contributing guide](CONTRIBUTING.md).

## License

Deckard is licensed under the [MIT License](LICENSE).

## Acknowledgments

We would like to thank the following people for their initial contributions building Deckard's first version:
- Lucas Soares: [@lucasoares](https://github.com/lucasoares)
- Gustavo Paiva: [@paivagustavo](https://github.com/paivagustavo)
- Cézar Augusto: [@cezar-tech](https://github.com/cezar-tech)
- Júnior Rhis: [@juniorrhis](https://github.com/juniorrhis)
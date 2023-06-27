# Contribute

You can help this project by reporting problems, suggestions or contributing to the code.

## Report a problem or suggestion

Go to our [issue tracker](https://github.com/takenet/deckard/issues) and check if your problem/suggestion is already reported. If not, create a new issue with a descriptive title and detail your suggestion or steps to reproduce the problem.

## Project structure and components

The project has as its base a RPC service with the Google implementation known as [gRPC](https://github.com/grpc) using [Protocol Buffers](https://developers.google.com/protocol-buffers).

It is organized in the following folders:

```
deckard                     # Root folder with project files and Golang service/gRPC generated sources
├── dashboards              # Dashboards templates for Grafana (metrics) and Kibana (audit)
├── docker                  # Docker compose to help running integration tests and base docker image file
├── helm                    # Helm chart for deploying Deckard in Kubernetes
├── docs                    # Documentation files
├── examples                # Examples using Deckard in different languages
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
├── java                    # Files for generating the Java client
├── csharp                  # Files for generating the C# client
├── proto                   # .proto files with the all definitions
```

> See the [configurations section](/README.md?#configuration) to see how to configure any internal component.

For more details about each component see the [components documentation](docs/components.md).

## Contribute to the code

To contribute with Deckard you need to follow these steps:

* Fork this repo using the button at the top.
* Clone your forked repo locally.

```shell
git clone git@github.com:yourname/deckard.git``
```

* Always create a new issue when you plan to work on a bug or new feature and wait for other devs input before start coding.
* Once the new feature is approved or the problem confirmed, go to your local copy and create a new branch to work on it. Use a descriptive name for it, include the issue number for reference.

```shell
git checkout -b message-title-3
```

* Do your coding and push it to your fork. Include as few commits as possible (one should be enough) and a good description. Always include a reference to the issue you are working on.

We currently use [Conventionnal Commits](https://www.conventionalcommits.org/en/v1.0.0/) to write our commit messages and our Pull Request title. This is a good practice to keep our commit history clean and easy to read.

```
$ git add .
$ git commit -m "feat(3): Adds a new title field to the message"
$ git push origin message-title-3
```

* Do a new pull request from your "message-title-3" branch to deckard "main" branch. Use the same title as the commit message and include a description of the changes you made.

### How to keep your local branches updated

To keep your local main branch updated with upstream main, regularly do:

```
$ git fetch upstream
$ git checkout main
$ git pull --rebase upstream main
```

To update the branch you are coding in:

```
$ git checkout message-title-3
$ git rebase main
```

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
> To change the default configuration see the [configuration section](/README.md?#configuration).

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

> We provide a simple [docker-compose](docker/docker-compose.yml) file to help you run these services.
>
> Run it with the following command:
> ```shell
> docker compose -f docker/docker-compose.yml up -d
> ```

Unit tests and integration tests may be found in the same file, but all integration tests must use the [short](https://golang.org/pkg/testing/#Short) flag.

Every integration tests have the suffix `Integration` in their name.

You will also need to generate certificates for the gRPC integration test. To do this, run the following command:
```shell
make gen-cert
```

> In order to be able to generate certificates, you need to have [openssl](https://www.openssl.org/) installed.
>
> Use `openssl version` to check if it is already installed.
>
> Check the [generation script](/internal/service/cert/gen.sh) for more details.

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

## Versioning

We use [Semantic Versioning](http://semver.org/) for versioning. To list all versions available, see the [tags on this repository](https://github.com/takenet/deckard/tags).

Our pre-release versions are named using the following pattern: `0.0.0-SNAPSHOT` and the `main` branch will always have a `SNAPSHOT` version. The `SNAPSHOT` stands for a code that was not released yet.

> Our tags are named using the following pattern: `v0.0.0`.

## Licesing

We currently use [MIT](LICENSE) license. Be careful to not include any code or dependency that is not compatible with this license.

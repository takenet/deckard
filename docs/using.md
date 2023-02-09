
# Using Deckard

Deckard is a service that provides a queue system for any application that needs to queue messages and prioritize them.

Since we use [gRPC](https://grpc.io/) you can use any language they support to communicate with Deckard.

> If you want to build a client for a language that we don't provide built-in support yet, you can use our [.proto file](proto/deckard_service.proto) file to generate the client following the [gRPC documentation](https://grpc.io/docs/languages/).

We currently provide built-in support for the following languages:
- [Golang](#golang)
- [Java](#java)

## Golang

To use Deckard in a Golang project you must first add Deckard to your project:
```shell
go get github.com/takenet/deckard
```

Then you can import the Deckard package and use it:
```golang
import (
    "github.com/takenet/deckard"
)
```

Check our [Golang example](../examples/golang/golang.go) for a complete example.

### Building the Golang API

Execute the following command to generate the Golang API files:
```shell
make gen-proto
```

## Java

We currently provide the Java API for the Deckard service in a Maven Repository.

> TODO: add maven repository and documentation on how to use it
> TODO: add example

### Building the Java API

To build java source files you must have [Maven](https://maven.apache.org/) installed.

Execute the following command to generate the Java API files:
```shell
make gen-java
```
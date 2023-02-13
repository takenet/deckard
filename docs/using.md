
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

Check our [Golang example](../examples/golang/example.go) for a complete example.

### Building the Golang API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the Golang API files again.

Execute the following command to generate the Golang API files:
```shell
make gen-proto
```

## Java

We currently provide the Java API for the Deckard service in a Maven Repository.

First you must add Deckard to your project. Check [here](https://github.com/takenet/deckard/packages/1790378) for the maven repository documentation.

After configuring the repository and adding the dependency you can import the Deckard package and use it. Check our [Java example](../examples/java/Example.java) for a complete example.

### Building the Java API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the Java API files again.

To build java source files you must have [Maven](https://maven.apache.org/) installed.

Execute the following command to generate the Java API files:
```shell
make gen-java
```

## C#

We currently provide the C# API for the Deckard service in a NuGet Repository.

First you must add Deckard to your project. Check [here](https://github.com/takenet/deckard/pkgs/nuget/Deckard) for the NuGet repository documentation.

Our repository link is `https://nuget.pkg.github.com/takenet/index.json` and you must configure it in your project (using `nuget.config` file or `dotnet nuget add source` command).

After configuring the repository you must add the dependency to your project using `dotnet add package` as documented [here](https://github.com/takenet/deckard/pkgs/nuget/Deckard). After that you can use the Deckard namespace. Check our [C# example](../examples/csharp/Example.cs) for a complete example.

### Building the C# API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the C# API files again.

Execute the following command to generate the C# API files:
```shell
make gen-csharp
```
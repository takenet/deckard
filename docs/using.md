
# Using Deckard

Deckard is a service that provides a queue system for any application that needs to queue messages and prioritize them.

Since we use [gRPC](https://grpc.io/) you can use any language they support to communicate with Deckard.

> If you want to build a client for a language that we don't provide built-in support yet, you can use our [.proto file](/proto/deckard_service.proto) file to generate the client following the [gRPC documentation](https://grpc.io/docs/languages/).

We currently provide built-in support for the following languages:
- [Golang](#golang)
- [Java](#java)
- [C#](#c)

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

**Check our [Golang example](../examples/golang/example.go) for a complete example.**

### Building the Golang API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the Golang API files again.

Execute the following command to generate the Golang API files:
```shell
make gen-proto
```

## Java

We currently provide the Java API for the Deckard service in the Maven Central Repository: https://central.sonatype.com/artifact/ai.blip/deckard

To add it to your project you must add the following repository to your `pom.xml` file:
```xml
<dependency>
  <groupId>ai.blip</groupId>
  <artifactId>deckard</artifactId>
  <version>${deckard.version}</version>
</dependency>
```

> Check the latest version and create the `deckard.version` property in your `pom.xml` file.

After adding the dependency you can import the Deckard package and use it.

**Check our [Java example](../examples/java/Example.java) for a complete example.**

### Building the Java API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the Java API files again.

To build java source files you must have [Maven](https://maven.apache.org/) installed.

Execute the following command to generate the Java API files:
```shell
make gen-java
```

## C#

We currently provide the C# API for the Deckard service in a NuGet Repository:
- https://www.nuget.org/packages/Deckard/

You must add the dependency using `dotnet add package` as documented [here](https://www.nuget.org/packages/Deckard/). After that you can use the Deckard namespace in your code.

**Check our [C# example](../examples/csharp/Example.cs) for a complete example.**

### Building the C# API

If you forked Deckard and changed anything in the [.proto](../proto/deckard_service.proto) file, you must generate the C# API files again.

Execute the following command to generate the C# API files:
```shell
make gen-csharp
```
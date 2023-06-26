# Getting Started

This guide will help you to start using Deckard and consists of the following sections:

- [Running Deckard](#running-deckard)
- [Consuming Deckard](#consuming-deckard)

## Running Deckard

The fastest way to start using deckard is by using Docker or downloading its binary.

If you want to execute it in a Kubernetes cluster you can use the Helm chart.

### Using docker

You can run deckard using docker by using the following command:

```bash
docker run --rm -p 8081:8081 blipai/deckard
```

> By default it will use a memory storage and a memory cache engine.
>
> To change the default configuration see the [configuration section](/README.md?#configuration).

### Using Helm

To install the Deckard chart, use the following commands:

```bash
helm repo add deckard https://takenet.github.io/deckard/
helm install deckard deckard/deckard
```

> It will deploy a MongoDB for storage and a Redis for cache.
>
> Check the chart [values.yaml](../helm/values.yaml) to see all available configurations.


### Using the binary

You can download the latest version of deckard from the [releases page](https://github.com/takenet/deckard/releases) or by using the following script:

`Linux Bash`
```bash
wget https://github.com/takenet/deckard/releases/latest/download/deckard-linux-amd64.tar.gz

tar -xzf deckard-linux-amd64.tar.gz

chmod +x deckard-linux-amd64

./deckard-linux-amd64
```

`Windows PowerShell`
```powershell
Invoke-WebRequest -Uri "https://github.com/takenet/deckard/releases/latest/download/deckard-windows-amd64.exe.zip" -OutFile "deckard-windows-amd64.exe.zip"

Expand-Archive -Path "deckard-windows-amd64.exe.zip" -DestinationPath .

Set-ExecutionPolicy -ExecutionPolicy Unrestricted -Force

.\deckard-windows-amd64.exe
```

## Consuming Deckard

Deckard exposes a gRPC service that can be consumed by any gRPC client. You can find the proto file [here](/proto/deckard_service.proto).

We suggest the usage of [Postman](https://www.postman.com/) to consume the gRPC service manually. You can follow the steps below to configure Postman to consume the gRPC service:

1. Install [Postman](https://www.postman.com/).
2. Open Postman and click on the `New` button:

![Postman New Button](/docs/postman/postman_new.png)

3. Select `gRPC Request`:

![Postman gRPC Request](/docs/postman/postman_grpc.png)

4. Fill the `gRPC Service URL` with the address of the Deckard instance and the methods will be disovered automatically using the reflection server:

![Postman gRPC Service URL](/docs/postman/postman_reflection.png)

5. Select the `Add` method to add messages and click `Generate Example Message`. Change the message body if wanted and click `Invoke`.

> Validate fields before sending, Postman may generate negative numbers for `int` fields.
>
> Also, Postman will not generate valid fields for the `payload` field. It will generate invalid `type_url` values. Check the [documentation](https://developers.google.com/protocol-buffers/docs/proto3#any) to see how `any` fields works and how to generate the `type_url` field.

You have now successfully added messages to the Deckard queue.

Now you can use the `Pull` method to consume messages and `Ack` or `Nack` to acknowledge the message. Remember to use the same `queue` when using `Pull` and the same `id` when using `Ack` or `Nack`.

## Next Steps

To learn how to configure deckard please check the [configuration section](/README.md?#configuration).

To learn how to use deckard in your application please check the [usage section](using.md) and enjoy Deckard in your favorite programming language.
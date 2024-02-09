using Grpc.Core;
using Grpc.Net.Client;
using Microsoft.Extensions.Options;

namespace Deckard.Benchmark.Api.Deckard;

public class DeckardClient : IDisposable
{
    public Takenet.Deckard.Deckard.DeckardClient Client { get; }
    private readonly GrpcChannel _channel;

    public DeckardClient
        (IOptions<DeckardConfiguration> deckardConfiguration, ILogger<DeckardClient> logger)
    {
        logger.LogInformation("Deckard address: {Address}", deckardConfiguration.Value.Address);

        _channel = GrpcChannel.ForAddress(
            deckardConfiguration.Value.Address,
            new GrpcChannelOptions { Credentials = ChannelCredentials.Insecure }
        );

        Client = new Takenet.Deckard.Deckard.DeckardClient(_channel);
    }

    void IDisposable.Dispose()
    {
        _channel.Dispose();
    }
}
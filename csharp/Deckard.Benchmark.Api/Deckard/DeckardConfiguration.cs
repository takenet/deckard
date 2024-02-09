using Deckard.Benchmark.Api.Worker;

namespace Deckard.Benchmark.Api.Deckard;

public class DeckardConfiguration
{
    public string Address { get; set; } = "http://localhost:8081";

    public DeckardWorkerConfiguration Worker { get; set; } = new();
}
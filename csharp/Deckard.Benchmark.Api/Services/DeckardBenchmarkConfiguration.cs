namespace Deckard.Benchmark.Api.Services;

public class DeckardBenchmarkConfiguration
{
    public Dictionary<string, long> organizationTPS { get; set; } = new();

    public long campaignTPS { get; set; } = new();
}
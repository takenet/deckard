namespace Deckard.Benchmark.Api.Worker;

public class DeckardWorkerConfiguration
{
    public TimeSpan PollingInterval { get; set; } = TimeSpan.FromSeconds(1);

    public int WorkerCount { get; set; } = 20;

    public TimeSpan PullTimeout { get; set; } = TimeSpan.FromSeconds(10);

    public PullRequestConfiguration PullRequest { get; set; } = new();
}
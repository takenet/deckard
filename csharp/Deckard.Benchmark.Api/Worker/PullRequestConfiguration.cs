namespace Deckard.Benchmark.Api.Worker;

public class PullRequestConfiguration
{
    public string QueueName { get; set; } = "organizations";

    public int Amount { get; set; } = 1;

    /// <summary>
    /// An auxiliary field to help create score filtering based on the timespan
    /// It will set the MaxScore to the current time minus the timespan
    /// It will filter messages with a score older than now minus the timespan
    /// It has precedence over MaxScore
    /// </summary>
    public TimeSpan MaxScoreTimestampFilter { get; set; } = TimeSpan.Zero;

    public double MaxScore { get; set; } = 0;

    public double MinScore { get; set; } = 0;

    /// <summary>
    /// Default: 1 minute
    /// </summary>
    public long AckDeadlineMs { get; set; } = 60_000;
}
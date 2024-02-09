using System.Diagnostics.Metrics;

namespace Deckard.Benchmark.Api.Metrics;

public class DeckardBenchmarkMetrics
{
    private Histogram<double> ExecutionHistogram { get; }

    public DeckardBenchmarkMetrics(IMeterFactory meterFactory)
    {
        var meter = meterFactory.Create("Blip.Deckard.Benchmark");

        ExecutionHistogram =
            meter.CreateHistogram<double>(
                "deckard.benchmark.execution",
                "ms",
                "The execution time of a task in the Deckard benchmark"
            );
    }

    public void Observe(TimeSpan time, string organizationId, string campaignId, string jobType) =>
        ExecutionHistogram.Record(
            time.TotalMilliseconds,
            new KeyValuePair<string, object?>("organizationId", organizationId),
            new KeyValuePair<string, object?>("campaignId", campaignId),
            new KeyValuePair<string, object?>("jobType", jobType)
        );
}
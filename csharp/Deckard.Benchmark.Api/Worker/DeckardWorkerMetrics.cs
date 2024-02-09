using System.Diagnostics.Metrics;

namespace Deckard.Benchmark.Api.Worker;

public class DeckardWorkerMetrics
{
    private Histogram<double> PullHistogram { get; }
    private Counter<long> EmptyResultsCounter { get; }
    private Counter<long> MessagesPulledCounter { get; }

    private Counter<long> ErrorsCounter { get; }

    public DeckardWorkerMetrics(IMeterFactory meterFactory)
    {
        var meter = meterFactory.Create("Blip.Deckard.Worker");

        PullHistogram =
            meter.CreateHistogram<double>(
                "deckard.worker.pull",
                "ms",
                "The execution time of a pull task in the Deckard Worker"
            );

        EmptyResultsCounter =
            meter.CreateCounter<long>(
                "deckard.worker.empty",
                "{pull}",
                "The number of empty pull responses from Deckard"
            );

        MessagesPulledCounter =
            meter.CreateCounter<long>(
                "deckard.worker.pulled",
                "{message}",
                "The number of messages pulled from Deckard"
            );

        ErrorsCounter =
            meter.CreateCounter<long>(
                "deckard.worker.errors",
                "The number of errors from pulling messages from Deckard"
            );
    }

    public void AddError(string queueName, long quantity = 1) =>
        ErrorsCounter.Add(
            quantity,
            new KeyValuePair<string, object?>("queue", queueName)
        );

    public void ObservePullTime(TimeSpan time, string queueName) =>
        PullHistogram.Record(
            time.TotalMilliseconds,
            new KeyValuePair<string, object?>("queue", queueName)
        );

    public void AddEmpty(string queueName, long quantity = 1) =>
        EmptyResultsCounter.Add(
            quantity,
            new KeyValuePair<string, object?>("queue", queueName)
        );

    public void AddPulled(string queueName, long quantity = 1) =>
        MessagesPulledCounter.Add(
            quantity,
            new KeyValuePair<string, object?>("queue", queueName)
        );
}
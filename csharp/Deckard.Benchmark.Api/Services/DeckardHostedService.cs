using Deckard.Benchmark.Api.Deckard;
using Deckard.Benchmark.Api.Metrics;
using Deckard.Benchmark.Api.Worker;
using Google.Protobuf;
using Microsoft.Extensions.Options;
using Takenet.Deckard;

namespace Deckard.Benchmark.Api.Services;

public class DeckardHostedService : IHostedService, IDisposable
{
    private readonly IOptions<DeckardBenchmarkConfiguration> _benchmarkConfiguration;
    private readonly DeckardBenchmarkMetrics _metrics;
    private readonly DeckardWorkerMetrics _workerMetrics;
    private readonly DeckardClient _client;
    private readonly ILogger<DeckardHostedService> _logger;
    private readonly DeckardWorker _worker;

    public DeckardHostedService(
        IOptions<DeckardBenchmarkConfiguration> benchmarkConfiguration,
        IOptions<DeckardWorkerConfiguration> configuration,
        DeckardBenchmarkMetrics metrics,
        DeckardWorkerMetrics workerMetrics,
        ILoggerFactory loggerFactory,
        DeckardClient client
    )
    {
        _benchmarkConfiguration = benchmarkConfiguration;
        _metrics = metrics;
        _workerMetrics = workerMetrics;
        _client = client;
        _logger = loggerFactory.CreateLogger<DeckardHostedService>();
        _worker = new DeckardWorker(configuration.Value, workerMetrics, loggerFactory, client, Process);
    }

    private void Process(PullResponse pullResponse)
    {
        var formatter = new JsonFormatter(JsonFormatter.Settings.Default);

        _logger.LogInformation("Processing messages {Messages}", formatter.Format(pullResponse));

        var organizationMessage = pullResponse.Messages.First();

        // Let other application get this organization again using the max score filter to configure TPS
        Ack(organizationMessage);

        // Get Next campaign to process
        var campaignMessages = _client.Client.Pull(new PullRequest
        {
            Amount = 1,
            Queue = $"campaigns::{organizationMessage.Id}",
            // TODO: set max score to configure organization-based TPS 
            MaxScore = DateTime.UtcNow.Subtract(TimeSpan.FromMilliseconds(100)).Millisecond
        });

        if (campaignMessages is null || campaignMessages.Messages.Count == 0)
        {
            // TODO after many empty responses, remove this organization from the organizations queue
            return;
        }

        var campaignMessage = campaignMessages.Messages.First();

        // Let other application get this campaign again using the max score filter to configure TPS
        Ack(campaignMessage);

        // Get next X audiences to process
        var audienceMessages = _client.Client.Pull(new PullRequest
        {
            Amount = 100,
            Queue = $"campaign::{campaignMessage.Id}",
            // Give 10 minutes to process all audiences
            AckDeadlineMs = 600_000,
            // TODO: set max score to configure campaign-based TPS 
        });

        if (audienceMessages is null || audienceMessages.Messages.Count == 0)
        {
            // TODO after many empty responses, remove this campaign from the campaigns queue
            return;
        }

        // Process each audience
        foreach (var audienceMessage in audienceMessages.Messages)
        {
            // After successful processing, ack the audience message, removing it from queue
            Ack(audienceMessage, remove: true);
        }
    }

    private bool Ack(Message organizationMessage, bool remove = false)
    {
        var ackResponse = _client.Client.Ack(new AckRequest
        {
            Id = organizationMessage.Id,
            Queue = organizationMessage.Queue,
            RemoveMessage = remove
        });

        if (ackResponse.Success)
        {
            _logger.LogInformation("Message {MessageId} from queue {Queue} was acked", organizationMessage.Id,
                organizationMessage.Queue);

            return true;
        }

        _logger.LogError("Message {MessageId} from queue {Queue} was not acked", organizationMessage.Id,
            organizationMessage.Queue);

        return false;
    }

    public Task StartAsync(CancellationToken cancellationToken)
    {
        return _worker.StartAsync(cancellationToken);
    }

    public Task StopAsync(CancellationToken cancellationToken)
    {
        return _worker.StopAsync(cancellationToken);
    }

    public void Dispose()
    {
        GC.SuppressFinalize(this);

        _worker.Dispose();
    }
}
using System.Diagnostics;
using System.Reactive.Linq;
using Deckard.Benchmark.Api.Deckard;
using OpenTelemetry.Trace;
using Takenet.Deckard;

namespace Deckard.Benchmark.Api.Worker;

public class DeckardWorker : IDisposable
{
    private readonly DeckardWorkerConfiguration _configuration;
    private readonly DeckardWorkerMetrics _metrics;
    private readonly ILogger<DeckardWorker> _logger;
    private readonly DeckardClient _client;
    private readonly Action<PullResponse> _action;

    private readonly Func<PullRequestConfiguration>? _pullParametersGetter;

    private readonly Action<PullRequest, Exception>? _onError;
    private readonly Action<PullRequest>? _onEmpty;
    private IDisposable? _subscription;

    /// <summary>
    /// </summary>
    /// <param name="configuration"></param>
    /// <param name="metrics"></param>
    /// <param name="loggerFactory"></param>
    /// <param name="client"></param>
    /// <param name="action"></param>
    /// <param name="pullParametersGetter">
    /// To be able to configure the pull request dynamically for each pull request.
    /// Defaults to the configuration.PullRequest
    /// </param>
    /// <param name="onError"></param>
    /// <param name="onEmpty"></param>
    /// <exception cref="ArgumentNullException"></exception>
    public DeckardWorker(
        DeckardWorkerConfiguration configuration,
        DeckardWorkerMetrics metrics,
        ILoggerFactory loggerFactory,
        DeckardClient client,
        Action<PullResponse> action,
        Func<PullRequestConfiguration>? pullParametersGetter = null,
        Action<PullRequest, Exception>? onError = null,
        Action<PullRequest>? onEmpty = null
    )
    {
        if (string.IsNullOrWhiteSpace(configuration.PullRequest.QueueName))
        {
            throw new ArgumentNullException(nameof(configuration),
                "QueueName from DeckardWorkerConfiguration.PullRequest is required");
        }

        _client = client;
        _action = action;
        _pullParametersGetter = pullParametersGetter;
        _onError = onError;
        _onEmpty = onEmpty;
        _configuration = configuration;
        _metrics = metrics;

        _logger = loggerFactory.CreateLogger<DeckardWorker>();
    }

    // TODO: implement backoff policy for empty and error responses
    // On error, back off exponentially after X errors until a max interval 
    // On empty, back off exponentially after X empty responses until a max interval
    private async Task<int> Pull()
    {
        if (string.IsNullOrWhiteSpace(_configuration.PullRequest.QueueName))
        {
            return 0;
        }

        PullResponse? response = null;

        var maxScore = _configuration.PullRequest.MaxScore;
        if (_configuration.PullRequest.MaxScoreTimestampFilter != TimeSpan.Zero)
        {
            maxScore = DateTimeOffset.UtcNow.Subtract(_configuration.PullRequest.MaxScoreTimestampFilter)
                .ToUnixTimeMilliseconds();
        }

        var pullRequest = new PullRequest
        {
            Amount = _configuration.PullRequest.Amount,
            Queue = _configuration.PullRequest.QueueName,
            MaxScore = maxScore,
            MinScore = _configuration.PullRequest.MinScore,
            AckDeadlineMs = _configuration.PullRequest.AckDeadlineMs
        };

        var stopwatch = Stopwatch.StartNew();
        try
        {
            response = await _client.Client.PullAsync(pullRequest,
                cancellationToken: new CancellationTokenSource(_configuration.PullTimeout).Token);
        }
        catch (Exception e)
        {
            _metrics.AddError(_configuration.PullRequest.QueueName);

            Activity.Current?.RecordException(e);

            if (_onError is null)
            {
                _logger.LogError(e, "Error pulling {Amount} messages from {QueueName}",
                    _configuration.PullRequest.Amount,
                    _configuration.PullRequest.QueueName);
            }
            else
            {
                _onError?.Invoke(pullRequest, e);
            }
        }
        finally
        {
            _metrics.ObservePullTime(stopwatch.Elapsed, _configuration.PullRequest.QueueName);
        }

        if (response is null || response.Messages.Count == 0)
        {
            _onEmpty?.Invoke(pullRequest);

            _metrics.AddEmpty(_configuration.PullRequest.QueueName);

            return 0;
        }

        _metrics.AddPulled(_configuration.PullRequest.QueueName, response.Messages.Count);

        _action(response);

        return response.Messages.Count;
    }

    public Task StartAsync(CancellationToken cancellationToken)
    {
        var query =
            Observable
                .Range(0, _configuration.WorkerCount)
                .Select(streamNumber =>
                    Observable
                        .Defer(() => Observable.Start(Pull))
                        .Delay(_configuration.PollingInterval)
                        .Repeat())
                .Merge();

        _subscription = query.Subscribe();

        return Task.CompletedTask;
    }

    public Task StopAsync(CancellationToken cancellationToken)
    {
        _subscription?.Dispose();
        _subscription = null;

        return Task.CompletedTask;
    }

    public void Dispose()
    {
        GC.SuppressFinalize(this);

        _subscription?.Dispose();
    }
}
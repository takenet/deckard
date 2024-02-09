using Deckard.Benchmark.Api.Deckard;
using Deckard.Benchmark.Api.Metrics;
using Deckard.Benchmark.Api.Services;
using Deckard.Benchmark.Api.Worker;
using OpenTelemetry.Metrics;
using OpenTelemetry.Resources;

var builder = WebApplication.CreateBuilder(args);

// Add services to the container.

builder.Services.AddControllers();
// Learn more about configuring Swagger/OpenAPI at https://aka.ms/aspnetcore/swashbuckle
builder.Services.AddEndpointsApiExplorer();
builder.Services.AddSwaggerGen();

ConfigureServices(builder);

var app = builder.Build();

app.UseOpenTelemetryPrometheusScrapingEndpoint();

// Configure the HTTP request pipeline.
if (app.Environment.IsDevelopment())
{
    app.UseSwagger();
    app.UseSwaggerUI();
}

app.UseHttpsRedirection();

app.UseAuthorization();

app.MapControllers();

app.Run();

return;

void ConfigureServices(WebApplicationBuilder webApplicationBuilder)
{
    webApplicationBuilder.Services.Configure<DeckardConfiguration>(
        webApplicationBuilder.Configuration.GetSection("Deckard")
    );

    webApplicationBuilder.Services.Configure<DeckardWorkerConfiguration>(
        webApplicationBuilder.Configuration.GetSection("Deckard").GetSection("Worker")
    );

    webApplicationBuilder.Services.Configure<DeckardBenchmarkConfiguration>(
        webApplicationBuilder.Configuration.GetSection("Benchmark")
    );

    webApplicationBuilder.Services.AddSingleton<DeckardClient>();

    webApplicationBuilder.Services.AddSingleton<DeckardBenchmarkMetrics>();
    webApplicationBuilder.Services.AddSingleton<DeckardWorkerMetrics>();

    webApplicationBuilder.Services.AddHostedService<DeckardHostedService>();

    webApplicationBuilder.Services.AddOpenTelemetry()
        .WithMetrics(meterProviderBuilder => meterProviderBuilder
            .SetResourceBuilder(ResourceBuilder.CreateDefault())
            .AddRuntimeInstrumentation()
            .AddProcessInstrumentation()
            .AddAspNetCoreInstrumentation()
            .AddMeter("Blip.*")
            .AddPrometheusExporter());
}
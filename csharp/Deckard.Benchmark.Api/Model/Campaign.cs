using System.Text.Json.Serialization;

namespace Deckard.Benchmark.Api.Model;

public class Campaign
{
    public string Id { get; set; }
    public string Name { get; set; }
    public string OrganizationId { get; set; }
    public int Audiences { get; set; }
}
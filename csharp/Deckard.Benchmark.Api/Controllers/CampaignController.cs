using Deckard.Benchmark.Api.Deckard;
using Deckard.Benchmark.Api.Model;
using Microsoft.AspNetCore.Mvc;
using Takenet.Deckard;

namespace Deckard.Benchmark.Api.Controllers;

[ApiController]
[Route("[controller]")]
public class CampaignController : ControllerBase
{
    private readonly ILogger<CampaignController> _logger;
    private readonly DeckardClient _deckardClient;

    public CampaignController(ILogger<CampaignController> logger, DeckardClient deckardClient)
    {
        _logger = logger;
        _deckardClient = deckardClient;
    }

    [HttpPost("generate")]
    public async Task<OkObjectResult> GenerateCampaigns([FromQuery] int campaignCount, [FromQuery] int audienceCount,
        [FromQuery] int organizationCount)
    {
        var generatorGuid = Guid.NewGuid().ToString();

        var tasks = new List<Task>();
        for (var oid = 0; oid < organizationCount; oid++)
        {
            for (var i = 0; i < campaignCount; i++)
            {
                var organizationId = "organization_" + oid;
                var campaignId = "campaign_" + Guid.NewGuid();
                var campaignName = $"{generatorGuid}_{organizationId}_{i}";

                var task = Task.Run((async () =>
                {
                    var campaign = new Campaign
                    {
                        Id = campaignId,
                        Name = campaignName,
                        Audiences = audienceCount,
                        OrganizationId = organizationId
                    };

                    await AddCampaign(campaign);
                }));

                tasks.Add(task);
            }
        }

        await Task.WhenAll(tasks);

        return Ok(new
        {
            Result = true
        });
    }

    [HttpPost("add")]
    public async Task<OkObjectResult> AddCampaign([FromBody] Campaign campaign)
    {
        campaign.Id = Guid.NewGuid().ToString();

        var batchList = new List<AddMessage>();
        for (var i = 0; i < campaign.Audiences; i++)
        {
            var message = new AddMessage
            {
                Id = $"{campaign.Id}_{i}",
                Queue = $"campaign::{campaign.Id}",
                // If not processed in 10 hours, discard
                TtlMinutes = 600,
                Metadata = { { "audience_id", i.ToString() }, { "campaign_id", campaign.Id } }
            };

            batchList.Add(message);

            if (batchList.Count % 10000 != 0 && i != campaign.Audiences - 1)
            {
                continue;
            }

            await Insert(campaign, batchList.ToArray());

            batchList.Clear();
        }

        var organizationMessage = new AddMessage
        {
            Id = $"{campaign.OrganizationId}",
            Queue = "organizations",
            Timeless = true,
        };

        var campaignMessage = new AddMessage
        {
            Id = $"{campaign.Id}",
            Queue = $"campaigns::{campaign.OrganizationId}",
            Timeless = true,
        };

        await Insert(campaign, organizationMessage, campaignMessage);

        return Ok(new
        {
            Result = true
        });
    }

    private async Task Insert(Campaign campaign, params AddMessage[] batchList)
    {
        var request = new AddRequest { Messages = { batchList } };

        var result = await _deckardClient.Client.AddAsync(request);

        if (result is null)
        {
            _logger.LogError("Failed to add messages to campaign {CampaignId}", campaign.Id);
        }
        else
        {
            _logger.LogInformation(
                "Added {AddedCount} and updated {UpdatedCount} messages to campaign {CampaignId}",
                result.CreatedCount, result.UpdatedCount, campaign.Id);
        }
    }
}
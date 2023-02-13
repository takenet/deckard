using Grpc.Net.Client;
using Takenet.Deckard;

using var channel = GrpcChannel.ForAddress(
    "localhost:8081",
    // Example using Insecure credentials
    new GrpcChannelOptions { Credentials = Grpc.Core.ChannelCredentials.Insecure }
);

var client = new Deckard.DeckardClient(channel);

// Create a new message
var message = new AddMessage{
    Id = "1",
    Queue = "queue",
};
message.Metadata.Add("key", "value");

var addRequest = new AddRequest();
addRequest.Messages.Add(message);

var addResponse = client.Add(addRequest);

if (addResponse.CreatedCount != 1)
{
    Console.WriteLine("Error adding message");

    Environment.Exit(1);
}

// Pull messages from the queue
var response = client.Pull(new PullRequest {
    Queue = "queue",
    Amount = 1
});

// Do something with the message
Console.WriteLine(response.Messages[0].Score);

// Ack message
client.Ack(new AckRequest {
    Id = response.Messages[0].Id,
    Queue = "queue",

    // Lock message for 10 seconds before it can be pulled again
    LockMs = 10000
});
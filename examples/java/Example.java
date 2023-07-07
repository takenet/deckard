package java;

import ai.blip.deckard.AckRequest;
import ai.blip.deckard.AddMessage;
import ai.blip.deckard.AddRequest;
import ai.blip.deckard.DeckardGrpc;
import ai.blip.deckard.PullRequest;
import io.grpc.ManagedChannelBuilder;

public class Example {
  public static void main(String[] args) {
    // Example using Insecure credentials
    var client =
        DeckardGrpc.newBlockingStub(
            ManagedChannelBuilder.forTarget("localhost:8081").usePlaintext().build());

    // Create a new message
    var addResponse =
        client.add(
            AddRequest.newBuilder()
                .addMessages(
                    AddMessage.newBuilder()
                        .setId("1")
                        .setQueue("queue")
                        .putMetadata("key", "value")
                        .build())
                .build());

    if (addResponse.getCreatedCount() != 1) {
      System.err.println("Message not added");

      System.exit(1);
    }

    // Pull messages from the queue
    var response = client.pull(PullRequest.newBuilder().setQueue("queue").setAmount(1).build());

    // Do something with the message
    System.out.println(response.getMessages(0).getScore());

    // Ack message
    client.ack(
        AckRequest.newBuilder()
            .setId(response.getMessages(0).getId())
            .setQueue("queue")
            // Remove message from queue
            .setRemoveMessage(true)
            // Lock message for 10 seconds before it can be pulled again
            //.setLockMs(10000L)
            .build());
  }
}

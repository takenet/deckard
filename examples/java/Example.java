package java;

import br.com.blipai.deckard.AckRequest;
import br.com.blipai.deckard.AddMessage;
import br.com.blipai.deckard.AddRequest;
import br.com.blipai.deckard.DeckardGrpc;
import br.com.blipai.deckard.PullRequest;
import io.grpc.ManagedChannelBuilder;

public class Example {
  public static void main(String[] args) {
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

            // Lock message for 10 seconds before it can be pulled again
            .setLockMs(10000L)
            .build());
  }
}

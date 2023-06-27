package main

import (
	"context"
	"log"
	"time"

	"github.com/takenet/deckard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Dial the connection
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Example using Insecure credentials
	conn, err := grpc.DialContext(ctx, "localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())

	if err != nil {
		log.Fatal(err)
	}

	client := deckard.NewDeckardClient(conn)
	defer conn.Close()

	// Create a new message
	response, err := client.Add(context.Background(), &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:         "1",
				Queue:      "queue",
				TtlMinutes: 10,
				Metadata:   map[string]string{"key": "value"},
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if response.CreatedCount != 1 {
		log.Fatal("Message not added")
	}

	// Pull messages from the queue
	pullResponse, err := client.Pull(context.Background(), &deckard.PullRequest{Queue: "queue", Amount: 1})
	if err != nil {
		log.Fatal(err)
	}

	// Do something with the message
	log.Println(pullResponse.Messages[0].Score)

	// Ack message
	client.Ack(context.Background(), &deckard.AckRequest{
		Id:    pullResponse.Messages[0].Id,
		Queue: "queue",

		// Remove the message from the queue
		RemoveMessage: true,

		// Lock message for 10 seconds before it can be pulled again
		//LockMs: 10000,
	})
}

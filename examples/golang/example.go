package main

import (
	"context"
	"log"
	"time"

	"github.com/takenet/deckard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// Example using Insecure credentials
	conn, err := grpc.NewClient("localhost:8081", grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	conn.Connect()
	readyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for state := conn.GetState(); state != connectivity.Ready; state = conn.GetState() {
		if !conn.WaitForStateChange(readyCtx, state) {
			log.Fatal("gRPC connection closed before becoming ready")
		}
	}

	client := deckard.NewDeckardClient(conn)

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

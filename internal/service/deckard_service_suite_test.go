package service

import (
	"context"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/queue"
	"github.com/takenet/deckard/internal/queue/cache"
	"github.com/takenet/deckard/internal/queue/score"
	"github.com/takenet/deckard/internal/queue/storage"
)

type DeckardIntegrationTestSuite struct {
	suite.Suite
	deckard        deckard.DeckardServer
	deckardQueue   queue.DeckardQueue
	deckardCache   cache.Cache
	deckardStorage storage.Storage
}

func (suite *DeckardIntegrationTestSuite) AfterTest(_, _ string) {
	suite.deckardCache.Flush(ctx)
	suite.deckardStorage.Flush(ctx)
}

func (suite *DeckardIntegrationTestSuite) BeforeTest(_, _ string) {
	suite.deckardCache.Flush(ctx)
	suite.deckardStorage.Flush(ctx)
}

func (suite *DeckardIntegrationTestSuite) TestAddMessageDefaultScoreIntegration() {
	start := time.Now()

	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:            "123",
				StringPayload: "Hello",
				Queue:         "test",
				Timeless:      true,
			},
		},
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), response.CreatedCount)

	messages, err := suite.deckardQueue.GetStorageMessages(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Ids: &[]string{"123"},
		},
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)

	result, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  "test",
		Amount: 1,
	})

	require.NoError(suite.T(), err)

	message := result.Messages[0]
	require.GreaterOrEqual(suite.T(), message.Score, score.GetScoreFromTime(&start))
	require.LessOrEqual(suite.T(), message.Score, score.GetScoreByDefaultAlgorithm())

	message.Score = 0
	message.Diagnostics = nil

	require.Equal(suite.T(), &deckard.Message{
		Id:            "123",
		Queue:         "test",
		StringPayload: "Hello",
	}, message)
}

func (suite *DeckardIntegrationTestSuite) TestAddMessageWithScoreIntegration() {
	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:       "123",
				Queue:    "test",
				Score:    100,
				Timeless: true,
			},
		},
	})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), response.CreatedCount)

	// Validate stored message
	messages, err := suite.deckardQueue.GetStorageMessages(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Queue: "test",
			Ids:   &[]string{"123"},
		},
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)
	require.Equal(suite.T(), float64(100), messages[0].Score)

	result, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  "test",
		Amount: 1,
	})

	require.NoError(suite.T(), err)

	message := result.Messages[0]
	require.Equal(suite.T(), float64(100), message.Score)

	message.Score = 0
	message.Diagnostics = nil

	require.Equal(suite.T(), &deckard.Message{
		Id:    "123",
		Queue: "test",
	}, message)
}

func (suite *DeckardIntegrationTestSuite) TestGetMessageIntegration() {
	start := time.Now()

	time.Sleep(10 * time.Millisecond)

	res, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:            "123",
				StringPayload: "Hello",
				Queue:         "test",
				Timeless:      false,
				TtlMinutes:    30,
			},
		},
	})

	require.NoError(suite.T(), err)

	count, err := suite.deckardQueue.Count(ctx, nil)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), count)
	require.Equal(suite.T(), int64(1), res.CreatedCount)

	messages, err := suite.deckardQueue.GetStorageMessages(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Ids: &[]string{"123"},
		},
	})

	require.NoError(suite.T(), err)

	doc := messages[0]

	require.Equal(suite.T(), "123", doc.ID)
	require.Equal(suite.T(), "Hello", doc.StringPayload)
	require.True(suite.T(), start.Add(30*time.Minute).Before(doc.ExpiryDate))
	require.True(suite.T(), start.Add(31*time.Minute).After(doc.ExpiryDate))
}

func (suite *DeckardIntegrationTestSuite) TestGetMessageShouldResultMostScoreFirstIntegration() {
	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{{
			Id:       "1",
			Queue:    "queue",
			Timeless: true,
		}, {
			Id:       "2",
			Queue:    "queue",
			Timeless: true,
		}, {
			Id:       "3",
			Queue:    "queue",
			Timeless: true,
		}},
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(3), response.CreatedCount)
	require.Equal(suite.T(), int64(0), response.UpdatedCount)

	count, err := suite.deckardQueue.Count(ctx, nil)
	require.Equal(suite.T(), int64(3), count)
	require.NoError(suite.T(), err)

	respose, err := suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 3, Queue: "queue"})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), 3, len(respose.Messages))

	first, err := suite.deckard.Ack(ctx, &deckard.AckRequest{
		Id:            "1",
		Queue:         "queue",
		ScoreSubtract: 5000,
	})

	require.NoError(suite.T(), err)
	require.True(suite.T(), first.GetSuccess())

	second, err := suite.deckard.Ack(ctx, &deckard.AckRequest{
		Id:            "2",
		Queue:         "queue",
		ScoreSubtract: 1000,
	})

	require.NoError(suite.T(), err)
	require.True(suite.T(), second.GetSuccess())

	third, err := suite.deckard.Ack(ctx, &deckard.AckRequest{
		Id:            "3",
		Queue:         "queue",
		ScoreSubtract: 90000,
	})

	require.NoError(suite.T(), err)
	require.True(suite.T(), third.GetSuccess())

	firstMessageResult, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  "queue",
		Amount: 1,
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), firstMessageResult.GetMessages(), 1)
	require.Equal(suite.T(), "3", firstMessageResult.GetMessages()[0].Id)

	secondMessageResult, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  "queue",
		Amount: 1,
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), secondMessageResult.GetMessages(), 1)
	require.Equal(suite.T(), "1", secondMessageResult.GetMessages()[0].Id)

	thirdMessageResult, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  "queue",
		Amount: 1,
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), thirdMessageResult.GetMessages(), 1)
	require.Equal(suite.T(), "2", thirdMessageResult.GetMessages()[0].Id)
}

func (suite *DeckardIntegrationTestSuite) TestPullMessageShouldContainsDiagnosticsIntegration() {
	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{{
			Id:       "1",
			Queue:    "queue",
			Timeless: true,
		}},
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), response.CreatedCount)
	require.Equal(suite.T(), int64(0), response.UpdatedCount)

	respose, err := suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 1, Queue: "queue"})
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), 1, len(respose.Messages))

	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.Acks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.Nacks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.ConsecutiveNacks)
}

func (suite *DeckardIntegrationTestSuite) TestIncrementDiagnosticsIntegration() {
	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{{
			Id:       "1",
			Queue:    "queue",
			Timeless: true,
		}},
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), response.CreatedCount)
	require.Equal(suite.T(), int64(0), response.UpdatedCount)

	// One Ack
	_, err = suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 1, Queue: "queue"})
	require.NoError(suite.T(), err)
	ack, err := suite.deckard.Ack(ctx, &deckard.AckRequest{Id: "1", Queue: "queue"})
	require.NoError(suite.T(), err)
	require.True(suite.T(), ack.GetSuccess())

	respose, err := suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 1, Queue: "queue"})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(1), respose.Messages[0].Diagnostics.Acks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.Nacks)
	require.Equal(suite.T(), int64(1), respose.Messages[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.ConsecutiveNacks)

	// One Nack
	_, err = suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 1, Queue: "queue"})
	require.NoError(suite.T(), err)
	nack, err := suite.deckard.Nack(ctx, &deckard.AckRequest{Id: "1", Queue: "queue"})
	require.NoError(suite.T(), err)
	require.True(suite.T(), nack.GetSuccess())

	respose, err = suite.deckard.Pull(ctx, &deckard.PullRequest{Amount: 1, Queue: "queue"})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(1), respose.Messages[0].Diagnostics.Acks)
	require.Equal(suite.T(), int64(1), respose.Messages[0].Diagnostics.Nacks)
	require.Equal(suite.T(), int64(0), respose.Messages[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(1), respose.Messages[0].Diagnostics.ConsecutiveNacks)
}

type AckNackAction = func(context.Context, *deckard.AckRequest) (*deckard.AckResponse, error)

func (suite *DeckardIntegrationTestSuite) TestShouldRemoveMessageAfterAnAckOrNack() {
	testMessageRemoval(suite, suite.deckard.Ack)
	testMessageRemoval(suite, suite.deckard.Nack)
}

func testMessageRemoval(suite *DeckardIntegrationTestSuite, ackOrNack AckNackAction) {
	queue := "remove_after_acknack"
	response, err := suite.deckard.Add(ctx, &deckard.AddRequest{
		Messages: []*deckard.AddMessage{
			{
				Id:            "-1",
				StringPayload: "Hello, remove me after the ack please",
				Queue:         queue,
				Timeless:      true,
			},
		},
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), response.CreatedCount)

	messages, err := suite.deckardQueue.GetStorageMessages(ctx, &storage.FindOptions{
		InternalFilter: &storage.InternalFilter{
			Ids: &[]string{"-1"},
		},
	})
	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)

	getMessageResp := suite.getMessagesFrom(queue)

	res := suite.performAckOrNack(&deckard.AckRequest{
		Queue: getMessageResp.Messages[0].GetQueue(),
		Id:    getMessageResp.Messages[0].GetId(),
	}, ackOrNack)

	require.True(suite.T(), res.GetRemovalResponse() == nil)

	getMessageResp = suite.getMessagesFrom(queue)

	res = suite.performAckOrNack(&deckard.AckRequest{
		Queue:         getMessageResp.Messages[0].GetQueue(),
		Id:            getMessageResp.Messages[0].GetId(),
		RemoveMessage: true,
		LockMs:        10000,
	}, ackOrNack)

	require.True(suite.T(), res.RemovalResponse.GetCacheRemoved() == 1)
	require.True(suite.T(), res.RemovalResponse.GetStorageRemoved() == 1)

	getMessageResp, err = suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  queue,
		Amount: 1,
	})

	require.NoError(suite.T(), err)
	require.True(suite.T(), len(getMessageResp.GetMessages()) == 0)

	countResp, err := suite.deckard.Count(ctx, &deckard.CountRequest{
		Queue: queue,
	})

	require.NoError(suite.T(), err)
	require.True(suite.T(), countResp.GetCount() == 0)
}

func (suite *DeckardIntegrationTestSuite) performAckOrNack(req *deckard.AckRequest, ackOrNack AckNackAction) *deckard.AckResponse {
	ackRes, err := ackOrNack(ctx, req)

	require.NoError(suite.T(), err)
	require.True(suite.T(), ackRes.GetSuccess())

	return ackRes
}

func (suite *DeckardIntegrationTestSuite) getMessagesFrom(queueName string) *deckard.PullResponse {
	getMessageResp, err := suite.deckard.Pull(ctx, &deckard.PullRequest{
		Queue:  queueName,
		Amount: 1,
	})
	require.NoError(suite.T(), err)
	require.True(suite.T(), len(getMessageResp.GetMessages()) == 1)
	return getMessageResp
}

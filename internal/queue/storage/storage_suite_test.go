package storage

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/elliotchance/orderedmap/v2"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/takenet/deckard"
	"github.com/takenet/deckard/internal/dtime"
	"github.com/takenet/deckard/internal/queue/configuration"
	"github.com/takenet/deckard/internal/queue/message"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var ctx = context.Background()

type StorageTestSuite struct {
	suite.Suite
	storage Storage
}

func (suite *StorageTestSuite) AfterTest(_, _ string) {
	_, err := suite.storage.Flush(ctx)

	if err != nil {
		panic(err)
	}
}

func (suite *StorageTestSuite) BeforeTest(_, _ string) {
	_, err := suite.storage.Flush(ctx)

	if err != nil {
		panic(err)
	}
}

func (suite *StorageTestSuite) insertDataNoError(messages ...*message.Message) {
	i, m, err := suite.storage.Insert(ctx, messages...)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(len(messages)), i)
	require.Equal(suite.T(), int64(0), m)
}

func (suite *StorageTestSuite) TestWithInternalFilterIdsOk() {
	messages := []*message.Message{{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q",
		ID:         "id2",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q",
		ID:         "id3",
		ExpiryDate: futureTime(),
	}}

	suite.insertDataNoError(messages...)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Ids: &[]string{"id", "id3"},
		},
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), result, 2)

	require.True(suite.T(), result[0].ID == "id" || result[0].ID == "id3")
	require.True(suite.T(), result[1].ID == "id" || result[1].ID == "id3")
}

func (suite *StorageTestSuite) TestAckShouldSetAckDiagnosticFields() {
	msg := &message.Message{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(msg)

	modified, err := suite.storage.Ack(ctx, msg)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), modified)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Ids:   &[]string{"id"},
			Queue: "q",
		},
	})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.Acks)
	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(0), *result[0].Diagnostics.ConsecutiveNacks)
}

func (suite *StorageTestSuite) TestNackShouldSetAckDiagnosticFields() {
	msg := &message.Message{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(msg)

	modified, err := suite.storage.Nack(ctx, msg)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), modified)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Ids:   &[]string{"id"},
			Queue: "q",
		},
	})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.Nacks)
	require.Equal(suite.T(), int64(0), *result[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.ConsecutiveNacks)
}

func (suite *StorageTestSuite) TestAckShouldIncrementCorrectly() {
	msg := &message.Message{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(msg)

	// Perform 5 Acks
	for i := 0; i < 5; i++ {
		_, err := suite.storage.Ack(ctx, msg)
		require.NoError(suite.T(), err)
	}

	// Perform 10 Nacks
	for i := 0; i < 10; i++ {
		_, err := suite.storage.Nack(ctx, msg)
		require.NoError(suite.T(), err)
	}

	// Another Ack
	_, err := suite.storage.Ack(ctx, msg)
	require.NoError(suite.T(), err)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Ids:   &[]string{"id"},
			Queue: "q",
		},
	})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(6), *result[0].Diagnostics.Acks)
	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(0), *result[0].Diagnostics.ConsecutiveNacks)
	require.Equal(suite.T(), int64(10), *result[0].Diagnostics.Nacks)
}

func (suite *StorageTestSuite) TestNackShouldIncrementCorrectly() {
	msg := &message.Message{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(msg)

	// Perform 5 Nacks
	for i := 0; i < 5; i++ {
		_, err := suite.storage.Nack(ctx, msg)
		require.NoError(suite.T(), err)
	}

	// Perform 10 Acks
	for i := 0; i < 10; i++ {
		_, err := suite.storage.Ack(ctx, msg)
		require.NoError(suite.T(), err)
	}

	// Another Nack
	_, err := suite.storage.Nack(ctx, msg)
	require.NoError(suite.T(), err)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Ids:   &[]string{"id"},
			Queue: "q",
		},
	})
	require.NoError(suite.T(), err)

	require.Equal(suite.T(), int64(6), *result[0].Diagnostics.Nacks)
	require.Equal(suite.T(), int64(1), *result[0].Diagnostics.ConsecutiveNacks)
	require.Equal(suite.T(), int64(0), *result[0].Diagnostics.ConsecutiveAcks)
	require.Equal(suite.T(), int64(10), *result[0].Diagnostics.Acks)
}

func (suite *StorageTestSuite) TestWithInternalFilterExpiryDateOk() {
	messages := []*message.Message{{
		Queue:      "q",
		ID:         "id",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q",
		ID:         "id2",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q",
		ID:         "id3",
		ExpiryDate: time.Now().Add(-10 * time.Hour),
	}}

	suite.insertDataNoError(messages...)

	now := time.Now()

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			ExpiryDate: &now,
		},
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), result, 1)

	require.Equal(suite.T(), result[0].ID, "id3")
}

func (suite *StorageTestSuite) TestWithInternalFilterQueueOk() {
	messages := []*message.Message{{
		Queue:      "q1",
		ID:         "id",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q2",
		ID:         "id2",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q2",
		ID:         "id3",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q3",
		ID:         "id4",
		ExpiryDate: futureTime(),
	}}

	suite.insertDataNoError(messages...)

	result, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			Queue: "q2",
		},
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), result, 2)

	require.True(suite.T(), result[0].ID == "id2" || result[0].ID == "id3")
	require.True(suite.T(), result[1].ID == "id2" || result[1].ID == "id3")
}

func (suite *StorageTestSuite) TestFindWithInternalFilterBreakpointOk() {
	messages := []*message.Message{{
		Queue:      "q1",
		ID:         "id",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q2",
		ID:         "id2",
		ExpiryDate: futureTime(),
	}, {
		Queue:      "q2",
		ID:         "id3",
		ExpiryDate: futureTime(),
	}}

	suite.insertDataNoError(messages...)

	orderedSort := orderedmap.NewOrderedMap[string, int]()
	orderedSort.Set("_id", 1)

	// Find all results sorted by internal id ascending
	result, err := suite.storage.Find(ctx, &FindOptions{
		Sort: orderedSort,
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), result, 3)

	// Get only elements with internal id bigger than result[1]
	filtered, err := suite.storage.Find(ctx, &FindOptions{
		InternalFilter: &InternalFilter{
			InternalIdBreakpointGt: suite.storage.GetStringInternalId(ctx, &result[1]),
		},
	})

	require.NoError(suite.T(), err)
	require.Len(suite.T(), filtered, 1)

	require.Equal(suite.T(), result[2], filtered[0])
}

func (suite *StorageTestSuite) TestInsertTwiceShouldReplaceMessageKeepingFields() {
	now := time.Now()

	msg := message.Message{
		Queue:      "q1",
		ID:         "id",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(&msg)

	ackModified, err := suite.storage.Ack(ctx, &message.Message{
		Queue:             "q1",
		ID:                "id",
		LastScoreSubtract: 1234.1,
		Breakpoint:        "12345",
		LastUsage:         &now,
		Score:             12345.1,
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), ackModified)

	newDate := futureTime().Add(time.Hour)
	newMessage := message.Message{
		Queue:         "q1",
		ID:            "id",
		ExpiryDate:    newDate,
		Description:   "newDescription",
		Metadata:      map[string]string{"new": "1234"},
		StringPayload: "newStringData",
	}

	i, m, err := suite.storage.Insert(ctx, &newMessage)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), m)
	require.Equal(suite.T(), int64(0), i)

	messages, err := suite.storage.Find(ctx, nil)

	require.NoError(suite.T(), err)

	data := messages[0]

	// Retain fields
	require.Equal(suite.T(), int64(1), data.UsageCount)
	require.Equal(suite.T(), 1234.1, data.TotalScoreSubtract)
	require.Equal(suite.T(), 1234.1, data.LastScoreSubtract)
	require.Equal(suite.T(), 12345.1, data.Score)
	require.Equal(suite.T(), dtime.MsPrecision(&now).Local(), dtime.MsPrecision(data.LastUsage).Local())
	require.Equal(suite.T(), "12345", data.Breakpoint)

	// New Data
	require.Equal(suite.T(), "newStringData", data.StringPayload)
	require.Equal(suite.T(), map[string]string{"new": "1234"}, data.Metadata)
	require.Equal(suite.T(), "newDescription", data.Description)
	require.Equal(suite.T(), dtime.MsPrecision(&newDate).Local(), dtime.MsPrecision(&data.ExpiryDate).Local())
}

func (suite *StorageTestSuite) TestInsertWithoutQueueShouldError() {
	messages := make([]*message.Message, 10)
	for i := range messages {
		messages[i] = &message.Message{ID: "123"}
	}

	_, _, err := suite.storage.Insert(ctx, messages...)

	require.Error(suite.T(), err, "message has a invalid queue")
}

func (suite *StorageTestSuite) TestInsertWithoutIDShouldError() {
	messages := make([]*message.Message, 10)
	for i := range messages {
		messages[i] = &message.Message{Queue: "123"}
	}

	_, _, err := suite.storage.Insert(ctx, messages...)

	require.Error(suite.T(), err, "message has a invalid ID")
}

func (suite *StorageTestSuite) TestUpdateOk() {
	msg := message.Message{
		ID:         "Id",
		Queue:      "Queue",
		ExpiryDate: futureTime(),
	}

	suite.insertDataNoError(&msg)

	now := time.Now()
	newTime := dtime.MsToTime(int64(1610300607851))
	firstAckModified, err := suite.storage.Ack(ctx, &message.Message{
		ID:                "Id",
		Queue:             "Queue",
		LastScoreSubtract: 123,
		Breakpoint:        "breakpoint1",
		LastUsage:         &now,
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), firstAckModified)

	secondAckModified, err := suite.storage.Ack(ctx, &message.Message{
		ID:                "Id",
		Queue:             "Queue",
		LastScoreSubtract: 54325,
		Breakpoint:        "breakpoint2",
		LastUsage:         &newTime,
	})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), secondAckModified)

	messages, err := suite.storage.Find(ctx, nil)

	require.NoError(suite.T(), err)
	require.Len(suite.T(), messages, 1)

	msTime := dtime.MsPrecision(&newTime).Local()

	require.Equal(suite.T(), msTime, dtime.MsPrecision(messages[0].LastUsage).Local())
	messages[0].LastUsage = nil
	msg.LastUsage = nil

	messages[0].ExpiryDate = dtime.MsPrecision(&messages[0].ExpiryDate).Local()
	messages[0].Diagnostics = nil

	require.Equal(suite.T(), message.Message{
		ID:                 "Id",
		Queue:              "Queue",
		ExpiryDate:         dtime.MsPrecision(&msg.ExpiryDate).Local(),
		InternalId:         messages[0].InternalId,
		Breakpoint:         "breakpoint2",
		LastScoreSubtract:  54325,
		TotalScoreSubtract: 54325 + 123,
		UsageCount:         2,
	}, messages[0])
}

func (suite *StorageTestSuite) TestListQueueNamesOk() {
	messages := make([]message.Message, 100)
	toInsert := make([]*message.Message, 100)

	queues := make([]string, 100)

	for i := range messages {
		queues[i] = strconv.Itoa(i)
		messages[i].Queue = strconv.Itoa(i)
		messages[i].ID = strconv.Itoa(i)

		toInsert[i] = &messages[i]
	}

	suite.insertDataNoError(toInsert...)

	resultQueues, err := suite.storage.ListQueueNames(ctx)

	require.NoError(suite.T(), err)

	sort.Strings(queues)
	sort.Strings(resultQueues)

	require.Equal(suite.T(), queues, resultQueues)
}

func (suite *StorageTestSuite) TestListQueueNamesShouldNotResultDeletedMessageQueue() {
	toInsert := make([]*message.Message, 100)
	queues := make([]string, 100)

	for i := 0; i < 100; i++ {
		message := message.Message{
			Queue: strconv.Itoa(i),
			ID:    strconv.Itoa(i),
		}

		queues[i] = strconv.Itoa(i)
		toInsert[i] = &message
	}

	suite.insertDataNoError(toInsert...)

	// Deleting one element from queue.
	// Deleting element from queue 1 because the result will be lexicography ordered and number 1 will be the second element
	removed, err := suite.storage.Remove(ctx, "1", "1")

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), removed)

	resultQueues, err := suite.storage.ListQueueNames(ctx)

	require.NoError(suite.T(), err)

	sort.Strings(queues)
	sort.Strings(resultQueues)

	require.NotEqual(suite.T(), queues, resultQueues)

	// Removing deleted element fom queue.
	queues = append(queues[:1], queues[2:]...)

	require.Equal(suite.T(), queues, resultQueues)
}

func (suite *StorageTestSuite) TestClearOk() {
	messages := make([]message.Message, 100)
	toInsert := make([]*message.Message, 100)

	for i := range messages {
		messages[i].Queue = "test"
		messages[i].ID = strconv.Itoa(i)

		toInsert[i] = &messages[i]
	}

	suite.insertDataNoError(toInsert...)

	count, err := suite.storage.Count(ctx, &FindOptions{})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(100), count)

	deleted, err := suite.storage.Flush(ctx)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(100), deleted)

	posCount, err := suite.storage.Count(ctx, &FindOptions{})

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(0), posCount)
}

func (suite *StorageTestSuite) TestClearShouldClearQueueConfigurations() {
	err := suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{
		Queue:       "queue",
		MaxElements: 1234,
	})
	require.NoError(suite.T(), err)

	deleted, err := suite.storage.Flush(ctx)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(1), deleted)

	config, err := suite.storage.GetQueueConfiguration(ctx, "queue")

	require.NoError(suite.T(), err)
	require.Nil(suite.T(), config)
}

func (suite *StorageTestSuite) TestInsertOneOk() {
	message := message.Message{
		ID:         "id",
		Queue:      "test",
		ExpiryDate: time.Now().Add(10 * time.Hour),
	}

	suite.insertDataNoError(&message)

	messages, err := suite.storage.Find(ctx, &FindOptions{InternalFilter: &InternalFilter{
		Ids:   &[]string{"id"},
		Queue: "test",
	}})

	require.NoError(suite.T(), err)

	require.Len(suite.T(), messages, 1, "expected one message")

	message.InternalId = messages[0].InternalId
	message.LastUsage = messages[0].LastUsage

	require.Equal(suite.T(), message.ExpiryDate.Unix(), messages[0].ExpiryDate.Unix())

	message.ExpiryDate = time.Time{}
	messages[0].ExpiryDate = time.Time{}

	require.Equal(suite.T(), message, messages[0])
	require.NotNil(suite.T(), message.LastUsage)
}

func (suite *StorageTestSuite) TestInsertWithPayloadOk() {
	ctx = context.Background()

	messageData := &deckard.AddMessage{
		Id: "rwefwef",
	}

	otherMessage, _ := anypb.New(messageData)
	stringData, _ := anypb.New(wrapperspb.String("some random string value"))
	stringEmptyData, _ := anypb.New(wrapperspb.String(""))
	intData, _ := anypb.New(wrapperspb.Int32(413))
	int0Data, _ := anypb.New(wrapperspb.Int32(0))
	boolData, _ := anypb.New(wrapperspb.Bool(true))
	boolFalseData, _ := anypb.New(wrapperspb.Bool(false))

	message := message.Message{
		ID:    "id",
		Queue: "test",
		Payload: map[string]*anypb.Any{
			"othermessage": otherMessage,
			"string":       stringData,
			"stringEmpty":  stringEmptyData,
			"int":          intData,
			"int0":         int0Data,
			"bool":         boolData,
			"boolFalse":    boolFalseData,
		},
		ExpiryDate: time.Now().Add(10 * time.Hour),
	}
	suite.insertDataNoError(&message)

	messages, err := suite.storage.Find(ctx, &FindOptions{InternalFilter: &InternalFilter{
		Ids:   &[]string{"id"},
		Queue: "test",
	}})

	require.NoError(suite.T(), err)

	resultMessage := messages[0]

	// check result value types
	stringValue := &wrapperspb.StringValue{}

	err = resultMessage.Payload["string"].UnmarshalTo(stringValue)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "some random string value", stringValue.GetValue())

	stringEmptyValue := &wrapperspb.StringValue{}
	err = resultMessage.Payload["stringEmpty"].UnmarshalTo(stringEmptyValue)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), "", stringEmptyValue.GetValue())

	intValue := &wrapperspb.Int32Value{}
	err = resultMessage.Payload["int"].UnmarshalTo(intValue)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int32(413), intValue.GetValue())

	int0Value := &wrapperspb.Int32Value{}
	err = resultMessage.Payload["int0"].UnmarshalTo(int0Value)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int32(0), int0Value.GetValue())

	boolValue := &wrapperspb.BoolValue{}
	err = resultMessage.Payload["bool"].UnmarshalTo(boolValue)
	require.NoError(suite.T(), err)
	require.True(suite.T(), boolValue.GetValue())

	boolFalseValue := &wrapperspb.BoolValue{}
	err = resultMessage.Payload["boolFalse"].UnmarshalTo(boolFalseValue)
	require.NoError(suite.T(), err)
	require.False(suite.T(), boolFalseValue.GetValue())

	otherMessageValue := &deckard.AddMessage{}
	err = resultMessage.Payload["othermessage"].UnmarshalTo(otherMessageValue)
	require.NoError(suite.T(), err)
	require.Equal(suite.T(), messageData.Id, otherMessageValue.Id)
	require.Equal(suite.T(), "rwefwef", otherMessageValue.Id)
}

func (suite *StorageTestSuite) TestInsertManyOk() {
	messages := make([]*message.Message, 200)

	for i := range messages {
		messages[i] = &message.Message{
			Queue:      "test",
			ID:         strconv.Itoa(i),
			ExpiryDate: time.Now().Add(time.Duration(i+1) * time.Hour),
		}
	}

	suite.insertDataNoError(messages...)

	orderedSort := orderedmap.NewOrderedMap[string, int]()
	orderedSort.Set("expiry_date", 1)

	data, err := suite.storage.Find(ctx, &FindOptions{
		Sort: orderedSort,
	})

	require.NoError(suite.T(), err)

	require.Len(suite.T(), data, 200, "expected 200 messages")

	// Comparing only with milliseconds precision for expiry date
	for i := range data {
		messages[i].InternalId = data[i].InternalId
		messages[i].LastUsage = data[i].LastUsage

		storageTime := dtime.MsPrecision(&messages[i].ExpiryDate).Local()
		messages[i].ExpiryDate = storageTime
		data[i].ExpiryDate = dtime.MsPrecision(&data[i].ExpiryDate).Local()

		require.Equal(suite.T(), *messages[i], data[i])
	}
}

func (suite *StorageTestSuite) TestGetConfigurationNotExistsShouldReturnNil() {
	config, err := suite.storage.GetQueueConfiguration(ctx, "queue2")

	require.NoError(suite.T(), err)
	require.Nil(suite.T(), config)
}

func (suite *StorageTestSuite) TestEditConfigurationShouldEditConfiguration() {
	err := suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: 2, Queue: "queue"})

	require.NoError(suite.T(), err)

	config, err := suite.storage.GetQueueConfiguration(ctx, "queue")

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(2), config.MaxElements)
}

func (suite *StorageTestSuite) TestListAllQueueConfigurations() {
	err := suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: 2, Queue: "queue"})
	require.NoError(suite.T(), err)

	err = suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: 43, Queue: "queue2"})
	require.NoError(suite.T(), err)

	queues, err := suite.storage.ListQueueConfigurations(ctx)

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), 2, len(queues))

	if queues[0].Queue == "queue" {
		require.Equal(suite.T(), configuration.QueueConfiguration{MaxElements: 2, Queue: "queue"}, *queues[0])
		require.Equal(suite.T(), configuration.QueueConfiguration{MaxElements: 43, Queue: "queue2"}, *queues[1])

	} else {
		require.Equal(suite.T(), configuration.QueueConfiguration{MaxElements: 2, Queue: "queue"}, *queues[1])
		require.Equal(suite.T(), configuration.QueueConfiguration{MaxElements: 43, Queue: "queue2"}, *queues[0])
	}

}

func (suite *StorageTestSuite) TestEditConfigurationWithNegativeNumberShouldMakeMaxElementsAsZero() {
	err := suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: -1, Queue: "queue"})

	require.NoError(suite.T(), err)

	config, err := suite.storage.GetQueueConfiguration(ctx, "queue")

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(0), config.MaxElements)
}

func (suite *StorageTestSuite) TestEditConfigurationWithMaxElementsZeroShouldDoNothing() {
	err := suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: 2, Queue: "queue"})

	require.NoError(suite.T(), err)

	err = suite.storage.EditQueueConfiguration(ctx, &configuration.QueueConfiguration{MaxElements: 0, Queue: "queue"})

	require.NoError(suite.T(), err)

	config, err := suite.storage.GetQueueConfiguration(ctx, "queue")

	require.NoError(suite.T(), err)
	require.Equal(suite.T(), int64(2), config.MaxElements)
}

func (suite *StorageTestSuite) TestFindWithNilOptionsOk() {
	message := message.Message{
		ID:         "id",
		Queue:      "test",
		ExpiryDate: time.Now().Add(10 * time.Hour),
	}

	suite.insertDataNoError(&message)

	data, err := suite.storage.Find(ctx, nil)

	require.NoError(suite.T(), err)

	require.Len(suite.T(), data, 1, "expected one message")

	message.InternalId = data[0].InternalId
	data[0].ExpiryDate = message.ExpiryDate
	data[0].LastUsage = message.LastUsage

	require.Equal(suite.T(), message, data[0])
}

func futureTime() time.Time {
	return time.Now().Add(10 * time.Hour)
}

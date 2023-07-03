package message

import (
	"strings"
	"time"

	"github.com/takenet/deckard/internal/queue/pool"
	"google.golang.org/protobuf/types/known/anypb"
)

// Message contains all data related to a single message. Including telemetry data.
type Message struct {
	ID string `json:"id" bson:"id"`

	// Description of the message, this should be used as a message's human readable id.
	Description string

	// Queue is the identifier of this message. Commonly used as channel.
	Queue string `json:"queue" bson:"queue"`

	// ExpiryDate is the date where this message is not valid anymore.
	ExpiryDate time.Time `json:"expiry_date,omitempty" bson:"expiry_date,omitempty"`

	// To ignore Expire Date and be eternal
	Timeless bool `json:"timeless" bson:"timeless"`

	// Metadata is a map of string to be used as a key-value store.
	// It is a simple way to store data that is not part of the message payload.
	Metadata map[string]string `json:"metadata,omitempty" bson:"metadata,omitempty"`

	// Payload is a structured data map that can be used to store any kind of data.
	// Is responsibility of the caller to know how to encode/decode to a useful format for its purpose.
	// It is recommended to use this field instead of Data.
	Payload map[string]*anypb.Any `json:"payload,omitempty" bson:"payload,omitempty"`

	// StringPayload is a field to populate with any custom data to store in the message.
	// This is represented as a string and is responsibility of the caller
	// to know how to encode/decode to a useful format for its purpose.
	StringPayload string `json:"string_payload,omitempty" bson:"string_payload,omitempty"`

	// Time in milliseconds that this message will be locked before returning to the message pool.
	LockMs int64 `json:"lock_ms" bson:"lock_ms"`

	// Internal Fields
	// Internal fields are managed by the Deckard itself and should not manually inserted on storage.

	// The internal storage id for this message
	// For MongoDB storage it is the _id field as primitive.ObjectID
	// For Memory storage it is an int64
	InternalId interface{} `json:"_id,omitempty" bson:"_id,omitempty"`

	// Score defines the score priority of this message.
	Score float64 `json:"score" bson:"score"`

	// Represents the result from the last time this message has been processed successfully (acknowledged).
	LastUsage         *time.Time `json:"last_usage,omitempty" bson:"last_usage,omitempty"`
	Breakpoint        string     `json:"breakpoint" bson:"breakpoint"`
	LastScoreSubtract float64    `json:"last_score_subtract" bson:"last_score_subtract"`

	// Statistical data about the 'performance' of this message.
	TotalScoreSubtract float64 `json:"total_score_subtract" bson:"total_score_subtract"`

	// Deprecated: use diagnostics.Ack instead
	UsageCount int64 `json:"usage_count" bson:"usage_count"`

	// Poppulated with the first part of the queue name if it contains the :: separator or the queue name otherwise.
	QueuePrefix string `json:"queue_prefix" bson:"queue_prefix"`

	// Poppulated with the second part of the queue name if it contains the :: separator.
	QueueSuffix string `json:"queue_suffix" bson:"queue_suffix"`

	// Diagnostics contains diagnostic data about this message.
	// Contains, for example, the number of times this message has been acked or nacked.
	Diagnostics *MessageDiagnostics `json:"diagnostics" bson:"diagnostics"`
}

func (q *Message) GetQueueParts() (prefix string, suffix string) {
	return GetQueueParts(q.Queue)
}

func GetQueueParts(queue string) (prefix string, suffix string) {
	if !strings.Contains(queue, pool.QUEUE_SEPARATOR) {
		return queue, ""
	}

	data := strings.SplitN(queue, pool.QUEUE_SEPARATOR, 2)

	return data[0], data[1]
}

func GetQueuePrefix(queue string) string {
	queuePrefix, _ := GetQueueParts(queue)

	return queuePrefix
}

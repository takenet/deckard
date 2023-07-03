package message

// MessageDiagnostics contains all diagnostic data related to a single message.
type MessageDiagnostics struct {
	Acks  *int64 `json:"acks" bson:"acks"`
	Nacks *int64 `json:"nacks" bson:"nacks"`

	ConsecutiveAcks  *int64 `json:"consecutive_acks" bson:"consecutive_acks"`
	ConsecutiveNacks *int64 `json:"consecutive_nacks" bson:"consecutive_nacks"`
}

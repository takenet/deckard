package configuration

type QueueConfiguration struct {
	// Queue is the identifier of this message.
	Queue string `json:"_id,omitempty" bson:"_id,omitempty"`

	// Max elements that a queue can have.
	MaxElements int64 `json:"max_elements,omitempty" bson:"max_elements,omitempty"`
}

package pool

type PoolType string

const (
	PRIMARY_POOL    PoolType = "primary_pool"
	PROCESSING_POOL PoolType = "processing_pool"
	LOCK_ACK_POOL   PoolType = "lock_ack_pool"
	LOCK_NACK_POOL  PoolType = "lock_nack_pool"
)

const QUEUE_SEPARATOR = "::"

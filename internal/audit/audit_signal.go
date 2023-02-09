package audit

type Signal string

const (
	ACK             Signal = "ACK"
	NACK            Signal = "NACK"
	TIMEOUT         Signal = "TIMEOUT"
	REMOVE          Signal = "REMOVE"
	UNLOCK          Signal = "UNLOCK"
	INSERT_CACHE    Signal = "INSERT_CACHE"
	MISSING_STORAGE Signal = "MISSING_STORAGE"
)

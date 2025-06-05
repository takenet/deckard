package config

import (
	"fmt"
	"os"
	"time"
)

var HousekeeperEnabled = Create(&ViperConfigKey{
	Key:     "housekeeper.enabled",
	Default: true,
})

var HousekeeperTaskTimeoutDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.timeout.delay",
	Default: "1s",
})

var HousekeeperTaskUnlockDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.unlock.delay",
	Default: "1s",
})

var HousekeeperTaskUpdateDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.update.delay",
	Default: "1s",
})

var HousekeeperTaskTTLDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.ttl.delay",
	Default: "1s",
})

var HousekeeperTaskMaxElementsDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.max_elements.delay",
	Default: "1s",
})

var HousekeeperTaskMetricsDelay = Create(&ViperConfigKey{
	Key:     "housekeeper.task.metrics.delay",
	Default: "60s",
})

var HousekeeperDistributedExecutionEnabled = Create(&ViperConfigKey{
	Key:     "housekeeper.distributed_execution.enabled",
	Default: false,
})

var HousekeeperDistributedExecutionInstanceID = Create(&ViperConfigKey{
	Key:     "housekeeper.distributed_execution.instance_id",
	Default: "",
})

var HousekeeperDistributedExecutionLockTTL = Create(&ViperConfigKey{
	Key:     "housekeeper.distributed_execution.lock_ttl",
	Default: "30s",
})

var HousekeeperUnlockParallelism = Create(&ViperConfigKey{
	Key:     "housekeeper.unlock.parallelism",
	Default: 5,
})

// GetHousekeeperInstanceID returns the instance ID for distributed execution
// If not provided in config, generates one based on hostname and timestamp
func GetHousekeeperInstanceID() string {
	configuredID := HousekeeperDistributedExecutionInstanceID.Get()
	if configuredID != "" {
		return configuredID
	}
	
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	
	return fmt.Sprintf("%s-%d", hostname, time.Now().Unix())
}

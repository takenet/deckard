package config

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

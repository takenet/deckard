package config

var AuditEnabled = Create(&ViperConfigKey{
	Key:     "audit.enabled",
	Default: false,
})

var ElasticAddress = Create(&ViperConfigKey{
	Key:     "elastic.address",
	Default: "http://localhost:9200/",
})

var ElasticPassword = Create(&ViperConfigKey{
	Key: "elastic.password",
})

var ElasticUser = Create(&ViperConfigKey{
	Key: "elastic.user",
})

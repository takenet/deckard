package config

var MetricsEnabled = Create(&ViperConfigKey{
	Key:     "metrics.enabled",
	Default: true,
})

var MetricsPort = Create(&ViperConfigKey{
	Key:     "metrics.port",
	Default: 22022,
})

var MetricsPath = Create(&ViperConfigKey{
	Key:     "metrics.path",
	Default: "/metrics",
})

var MetricsHistogramBuckets = Create(&ViperConfigKey{
	Key:     "metrics.histogram.buckets",
	Default: "0,1,2,5,10,15,20,30,35,50,100,200,400,600,800,1000,1500,2000,5000,10000",
})

var MetricsOpenMetricsEnabled = Create(&ViperConfigKey{
	Key:     "metrics.openmetrics.enabled",
	Default: true,
})

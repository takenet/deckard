package config

var GrpcEnabled = Create(&ViperConfigKey{
	Key:     "grpc.enabled",
	Default: true,
})

var GrpcPort = Create(&ViperConfigKey{
	Key:     "grpc.port",
	Default: 8081,
})

var GrpcServerKeepaliveTime = Create(&ViperConfigKey{
	Key: "grpc.server.keepalive_time",
})

var GrpcServerKeepaliveTimeout = Create(&ViperConfigKey{
	Key: "grpc.server.keepalive_timeout",
})

var GrpcServerMaxConnectionIdle = Create(&ViperConfigKey{
	Key: "grpc.server.max_connection_idle",
})

var GrpcServerMaxConnectionAge = Create(&ViperConfigKey{
	Key: "grpc.server.max_connection_age",
})

var GrpcServerMaxConnectionAgeGrace = Create(&ViperConfigKey{
	Key: "grpc.server.max_connection_age_grace",
})

var TlsServerCertFilePaths = Create(&ViperConfigKey{
	Key: "tls.server.cert_file_paths",
})

var TlsServerKeyFilePaths = Create(&ViperConfigKey{
	Key: "tls.server.key_file_paths",
})

var TlsClientCertFilePaths = Create(&ViperConfigKey{
	Key: "tls.client.cert_file_paths",
})

var TlsClientAuthType = Create(&ViperConfigKey{
	Key:     "tls.client.auth_type",
	Default: "NoClientCert",
})

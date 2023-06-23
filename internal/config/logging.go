package config

import "github.com/spf13/viper"

var DebugEnabled = Create(&ViperConfigKey{
	Key:     "debug",
	Default: false,
})

var LogType = Create(&ViperConfigKey{
	Key:     "log_type",
	Default: "json",
})

func init() {
	viper.BindEnv(DebugEnabled.GetKey(), "DEBUG", "debug")
	viper.BindEnv(LogType.GetKey(), "LOG_TYPE", "log_type")
}

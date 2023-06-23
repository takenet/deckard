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
	_ = viper.BindEnv(DebugEnabled.GetKey(), "DEBUG", "debug")
	_ = viper.BindEnv(LogType.GetKey(), "LOG_TYPE", "log_type")
}

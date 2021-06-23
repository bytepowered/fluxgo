package server

import (
	"context"
	"os"
)

import (
	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

import (
	"github.com/bytepowered/fluxgo/pkg/ext"
	"github.com/bytepowered/fluxgo/pkg/flux"
	"github.com/bytepowered/fluxgo/pkg/logger"
)

const (
	EnvKeyDeployEnv = "DEPLOY_ENV"
)

func InitLogger() {
	config, err := logger.LoadConfig("")
	if nil != err {
		panic(err)
	}
	sugar := logger.NewZapLogger(config)
	logger.SetSimpleLogger(sugar)
	zap.ReplaceGlobals(sugar.Desugar())
	ext.SetLoggerFactory(func(values context.Context) flux.Logger {
		if nil == values {
			return sugar
		}
		if traceId := values.Value(logger.TraceId); nil != traceId {
			return sugar.With(zap.String(logger.TraceId, cast.ToString(traceId)))
		}
		return sugar
	})
}

func InitAppConfig(envKey string) {
	file := "application"
	env := os.Getenv(envKey)
	if env != "" {
		file = file + "-" + env
	}
	viper.SetConfigName(file)
	viper.AddConfigPath("/etc/flux/conf.d")
	viper.AddConfigPath("./conf.d")
	logger.Infof("Using config, file: %s, Env: %s", file, env)
	if err := viper.ReadInConfig(); nil != err {
		logger.Panicw("Fatal config error", "path", file, "error", err)
	}
}

func IsDisabled(config *flux.Configuration) bool {
	return config.GetBool("disable") || config.GetBool("disabled")
}

func DefaultRequestVersionLocateFunc(webex flux.WebContext) (version string) {
	return webex.HeaderVar(DefaultHttpHeaderVersion)
}
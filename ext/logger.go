package ext

import (
	"context"
	"github.com/bytepowered/flux"
)

var (
	loggerFactory flux.LoggerFactory
)

func SetLoggerFactory(f flux.LoggerFactory) {
	flux.AssertNotNil(f, "LoggerFactory must not nil")
	loggerFactory = f
}

// NewLoggerWith
func NewLoggerWith(ctx context.Context) flux.Logger {
	return loggerFactory(ctx)
}

// NewLogger ...
func NewLogger() flux.Logger {
	return NewLoggerWith(context.TODO())
}

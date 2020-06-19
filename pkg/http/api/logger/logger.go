package logger

import (
	"go.uber.org/zap"
)

var log *zap.SugaredLogger

func Debugf(fmt string, args ...interface{}) {
	log.Debugf(fmt, args...)
}

func Fatalf(fmt string, args ...interface{}) {
	log.Fatalf(fmt, args...)
}

func Errorf(fmt string, args ...interface{}) {
	log.Errorf(fmt, args...)
}

func Infof(fmt string, args ...interface{}) {
	log.Infof(fmt, args...)
}

func Setup(level string) {
	//TODO: map from env config
	var zlog *zap.Logger
	switch level {
	case "debug":
		zlog, _ = zap.NewDevelopment()
	case "none":
		zlog = zap.NewNop()
	default:
		zlog, _ = zap.NewProduction()
	}
	// This condition is to avoid race conditions in test cases
	if log == nil {
		log = zlog.Sugar()
	}
}

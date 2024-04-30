package run

import (
	"io"

	log "github.com/sirupsen/logrus"
)

type NullLogger struct {}

func (n NullLogger) WithJobLogger() *log.Logger {
	logger := log.New()
	logger.SetOutput(io.Discard)
	logger.SetLevel(log.TraceLevel)

	return logger
}

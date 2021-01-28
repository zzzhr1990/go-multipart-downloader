package model

import "log"

// ILogger logger
type ILogger interface {
	// Infof logs a message at level Info on the standard logger.
	Infof(format string, args ...interface{})

	// Errorf logs a message at level Error on the standard logger.
	Errorf(format string, args ...interface{})
}

// DefaultLogger default logger if not define
type DefaultLogger struct {
}

// Infof ifo
func (*DefaultLogger) Infof(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// Errorf err
func (*DefaultLogger) Errorf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

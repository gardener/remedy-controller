package config

import (
	log "github.com/sirupsen/logrus"
)

// ConfigureLogger configures the Logger. The info log level will be ensured if no valid log level passed.
func ConfigureLogger(level string) {
	// Format log output.
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
		DisableColors: true,
	})

	// Set the log level.
	switch level {
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "debug":
		log.SetLevel(log.DebugLevel)
	default:
		log.Infof("Log level %s can't be applied. Use info log level.", level)
		log.SetLevel(log.InfoLevel)
	}
}

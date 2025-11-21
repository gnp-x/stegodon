//go:build !linux
// +build !linux

package util

import (
	"io"
	"log"
	"os"
)

var logWriter io.Writer = os.Stderr

// GetLogWriter returns the current log writer (for use by other packages)
func GetLogWriter() io.Writer {
	return logWriter
}

// SetupLogging configures the logging system based on the journald flag
// On non-Linux systems, journald is not available, so we use standard logging
func SetupLogging(withJournald bool) {
	if withJournald {
		log.Println("Warning: Journald logging is not supported on this operating system")
		log.Println("Falling back to standard logging (stdout/stderr)")
	}
	// Use default logging regardless (stdout/stderr with timestamps)
}

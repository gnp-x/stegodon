//go:build linux
// +build linux

package util

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/coreos/go-systemd/v22/journal"
)

// journaldWriter implements io.Writer for journald logging
type journaldWriter struct{}

func (w *journaldWriter) Write(p []byte) (n int, err error) {
	// Send to journald with INFO priority
	// Remove trailing newline if present (journald adds its own)
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	// Send to journald with proper syslog identifier
	err = journal.Send(msg, journal.PriInfo, map[string]string{
		"SYSLOG_IDENTIFIER": "stegodon",
	})
	if err != nil {
		// If journald write fails, fall back to stderr
		return fmt.Fprintf(os.Stderr, "%s", p)
	}
	return len(p), nil
}

var logWriter io.Writer = os.Stderr

// GetLogWriter returns the current log writer (for use by other packages)
func GetLogWriter() io.Writer {
	return logWriter
}

// SetupLogging configures the logging system based on the journald flag
func SetupLogging(withJournald bool) {
	if withJournald {
		// Check if journald is available
		if !journal.Enabled() {
			log.Println("Warning: Journald not available on this system; using standard logging")
			return
		}

		// Set up journald writer
		writer := &journaldWriter{}
		logWriter = writer
		log.SetOutput(writer)
		log.SetFlags(0) // journald adds its own timestamps
		log.Println("Logging initialized with journald support")
	}
	// If withJournald is false, use default logging (stdout/stderr)
}

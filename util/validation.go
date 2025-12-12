package util

import (
	"regexp"
	"unicode"
)

// Pre-compiled regex for WebFinger username validation
var webFingerValidCharsRegex = regexp.MustCompile(`^[A-Za-z0-9\-._~!$&'()*+,;=]+$`)

// IsValidWebFingerUsername validates that a username meets WebFinger/ActivityPub requirements.
//
// WebFinger allows these characters without percent-encoding:
// A-Z a-z 0-9 - . _ ~ ! $ & ' ( ) * + , ; =
//
// Any other Unicode character (like ä, 字, 🔥) must be percent-encoded and is rejected here.
// Non-printable/control characters are also rejected.
//
// Returns (true, "") if valid, or (false, "error message") if invalid.
func IsValidWebFingerUsername(username string) (bool, string) {
	if len(username) == 0 {
		return false, "Username must be at least 1 character"
	}

	// Check for valid WebFinger characters (no Unicode, no spaces, no special chars except allowed set)
	// Allowed: A-Z a-z 0-9 - . _ ~ ! $ & ' ( ) * + , ; =
	if !webFingerValidCharsRegex.MatchString(username) {
		return false, "Username contains invalid characters. Only A-Z, a-z, 0-9, and -._~!$&'()*+,;= are allowed"
	}

	// Check for control characters (shouldn't match regex above, but double-check)
	for _, r := range username {
		if unicode.IsControl(r) || !unicode.IsPrint(r) {
			return false, "Username contains non-printable characters"
		}
	}

	return true, ""
}

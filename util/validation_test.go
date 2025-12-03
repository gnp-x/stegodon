package util

import (
	"strings"
	"testing"
)

func TestIsValidWebFingerUsername(t *testing.T) {
	tests := []struct {
		username string
		valid    bool
		errMsg   string
	}{
		// Valid usernames
		{"alice", true, ""},
		{"alice123", true, ""},
		{"alice-bob", true, ""},
		{"alice.bob_123", true, ""},
		{"alice_bob~test", true, ""},
		{"alice!test", true, ""},
		{"alice$test", true, ""},
		{"alice&test", true, ""},
		{"alice'test", true, ""},
		{"alice(bob)", true, ""},
		{"alice*bob+charlie", true, ""},
		{"alice,bob;charlie", true, ""},
		{"alice=bob", true, ""},
		{"test!$&'()*+,;=123", true, ""}, // All allowed special chars

		// Invalid usernames - empty
		{"", false, "must be at least 1 character"},

		// Invalid usernames - Unicode characters
		{"älice", false, "invalid characters"},
		{"alice_ö", false, "invalid characters"},
		{"字", false, "invalid characters"},
		{"test字test", false, "invalid characters"},

		// Invalid usernames - Emoji
		{"alice🔥", false, "invalid characters"},
		{"🔥", false, "invalid characters"},
		{"test🔥test", false, "invalid characters"},

		// Invalid usernames - spaces
		{"alice bob", false, "invalid characters"},
		{" alice", false, "invalid characters"},
		{"alice ", false, "invalid characters"},

		// Invalid usernames - control characters
		{"alice\n", false, "invalid characters"},
		{"alice\t", false, "invalid characters"},
		{"alice\r", false, "invalid characters"},
		{"\nalice", false, "invalid characters"},

		// Invalid usernames - other special characters not in allowed set
		{"alice@bob", false, "invalid characters"}, // @ not allowed
		{"alice#bob", false, "invalid characters"}, // # not allowed
		{"alice%bob", false, "invalid characters"}, // % not allowed
		{"alice^bob", false, "invalid characters"}, // ^ not allowed
		{"alice[bob]", false, "invalid characters"}, // [] not allowed
		{"alice{bob}", false, "invalid characters"}, // {} not allowed
		{"alice|bob", false, "invalid characters"}, // | not allowed
		{"alice\\bob", false, "invalid characters"}, // \ not allowed
		{"alice/bob", false, "invalid characters"}, // / not allowed
		{"alice:bob", false, "invalid characters"}, // : not allowed
		{"alice<bob>", false, "invalid characters"}, // <> not allowed
		{"alice?bob", false, "invalid characters"}, // ? not allowed
	}

	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			valid, errMsg := IsValidWebFingerUsername(tt.username)

			if valid != tt.valid {
				t.Errorf("Expected valid=%v, got %v for username '%s'", tt.valid, valid, tt.username)
			}

			if !tt.valid && tt.errMsg != "" && !strings.Contains(strings.ToLower(errMsg), strings.ToLower(tt.errMsg)) {
				t.Errorf("Expected error containing '%s', got '%s' for username '%s'", tt.errMsg, errMsg, tt.username)
			}

			if tt.valid && errMsg != "" {
				t.Errorf("Expected no error for valid username '%s', got '%s'", tt.username, errMsg)
			}
		})
	}
}

func TestIsValidWebFingerUsername_EdgeCases(t *testing.T) {
	// Test very long username (should be valid if only contains valid chars)
	longUsername := strings.Repeat("a", 100)
	valid, _ := IsValidWebFingerUsername(longUsername)
	if !valid {
		t.Error("Expected very long username with valid chars to be valid")
	}

	// Test single character usernames with each allowed char type
	singleCharTests := []string{"a", "Z", "0", "9", "-", ".", "_", "~", "!", "$", "&", "'", "(", ")", "*", "+", ",", ";", "="}
	for _, char := range singleCharTests {
		valid, errMsg := IsValidWebFingerUsername(char)
		if !valid {
			t.Errorf("Expected single character '%s' to be valid, got error: %s", char, errMsg)
		}
	}
}

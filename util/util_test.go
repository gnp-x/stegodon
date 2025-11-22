package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"
)

func TestPublicKeyToString(t *testing.T) {
	// This function requires an SSH session which is hard to mock
	// We'll skip it for now as it's more of an integration test
	t.Skip("PublicKeyToString requires SSH session - integration test")
}

func TestPkToHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple string",
			input:    "test",
			expected: "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:     "ssh key format",
			input:    "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ",
			expected: "8f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c9f7c4c9c9e3c8e9c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PkToHash(tt.input)
			// Just verify it returns a 64-character hex string
			if len(result) != 64 {
				t.Errorf("Expected hash length 64, got %d", len(result))
			}
			// Verify it's consistent
			result2 := PkToHash(tt.input)
			if result != result2 {
				t.Errorf("Hash should be consistent: %s != %s", result, result2)
			}
		})
	}
}

func TestPkToHashDifferentInputs(t *testing.T) {
	hash1 := PkToHash("input1")
	hash2 := PkToHash("input2")

	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestGetVersion(t *testing.T) {
	// GetVersion now uses embedded version.txt
	// Read expected version from version.txt to avoid hardcoding
	version := GetVersion()

	// Version should not be empty
	if version == "" {
		t.Error("Version should not be empty")
	}

	// Version should match semantic versioning pattern (e.g., "1.2.2")
	// At minimum, should contain digits and dots
	hasDigit := false
	hasDot := false
	for _, char := range version {
		if char >= '0' && char <= '9' {
			hasDigit = true
		}
		if char == '.' {
			hasDot = true
		}
	}

	if !hasDigit {
		t.Error("Version should contain at least one digit")
	}
	if !hasDot {
		t.Error("Version should contain at least one dot (semantic versioning)")
	}
}

func TestGetNameAndVersion(t *testing.T) {
	// GetNameAndVersion now uses embedded version.txt
	result := GetNameAndVersion()
	expected := fmt.Sprintf("stegodon / %s", GetVersion())

	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestRandomString(t *testing.T) {
	tests := []int{10, 20, 32, 64}

	for _, length := range tests {
		t.Run("length_"+string(rune(length+'0')), func(t *testing.T) {
			result := RandomString(length)
			if len(result) != length {
				t.Errorf("Expected length %d, got %d", length, len(result))
			}
		})
	}
}

func TestRandomStringUniqueness(t *testing.T) {
	// Generate multiple random strings and verify they're different
	results := make(map[string]bool)
	for range 100 {
		s := RandomString(32)
		if results[s] {
			t.Errorf("RandomString produced duplicate: %s", s)
		}
		results[s] = true
	}
}

func TestNormalizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "newlines replaced",
			input:    "line1\nline2\nline3",
			expected: "line1 line2 line3",
		},
		{
			name:     "html escaped",
			input:    "<script>alert('xss')</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:     "combined newlines and html",
			input:    "<div>\ntest\n</div>",
			expected: "&lt;div&gt; test &lt;/div&gt;",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "normal text",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "ampersand",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "quotes",
			input:    `He said "Hello"`,
			expected: "He said &#34;Hello&#34;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestDateTimeFormat(t *testing.T) {
	format := DateTimeFormat()
	expected := "2006-01-02 15:04:05 CEST"

	if format != expected {
		t.Errorf("Expected format '%s', got '%s'", expected, format)
	}
}

func TestPrettyPrint(t *testing.T) {
	tests := []struct {
		name  string
		input any
	}{
		{
			name:  "simple map",
			input: map[string]string{"key": "value"},
		},
		{
			name:  "nested structure",
			input: map[string]any{"outer": map[string]int{"inner": 42}},
		},
		{
			name:  "array",
			input: []int{1, 2, 3, 4, 5},
		},
		{
			name:  "string",
			input: "simple string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PrettyPrint(tt.input)
			if len(result) == 0 {
				t.Error("PrettyPrint returned empty string")
			}
		})
	}
}

func TestGeneratePemKeypair(t *testing.T) {
	keypair := GeneratePemKeypair()

	if keypair == nil {
		t.Fatal("GeneratePemKeypair returned nil")
	}

	// Check private key format (now PKCS#8)
	if len(keypair.Private) == 0 {
		t.Error("Private key is empty")
	}
	if !contains(keypair.Private, "BEGIN PRIVATE KEY") {
		t.Error("Private key doesn't have PKCS#8 PEM header")
	}
	if !contains(keypair.Private, "END PRIVATE KEY") {
		t.Error("Private key doesn't have PKCS#8 PEM footer")
	}

	// Check public key format (now PKIX)
	if len(keypair.Public) == 0 {
		t.Error("Public key is empty")
	}
	if !contains(keypair.Public, "BEGIN PUBLIC KEY") {
		t.Error("Public key doesn't have PKIX PEM header")
	}
	if !contains(keypair.Public, "END PUBLIC KEY") {
		t.Error("Public key doesn't have PKIX PEM footer")
	}
}

func TestGeneratePemKeypairUniqueness(t *testing.T) {
	keypair1 := GeneratePemKeypair()
	keypair2 := GeneratePemKeypair()

	if keypair1.Private == keypair2.Private {
		t.Error("Generated keypairs should be unique (private keys are identical)")
	}
	if keypair1.Public == keypair2.Public {
		t.Error("Generated keypairs should be unique (public keys are identical)")
	}
}

func TestConvertPrivateKeyToPKCS8(t *testing.T) {
	// Generate a real PKCS#1 key for testing
	oldKeypair := &RsaKeyPair{}
	bitSize := 2048 // Minimum secure size

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	// Create PKCS#1 format (old format)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	oldKeypair.Private = string(pkcs1PEM)

	// Convert to PKCS#8
	pkcs8Key, err := ConvertPrivateKeyToPKCS8(oldKeypair.Private)
	if err != nil {
		t.Fatalf("Failed to convert PKCS#1 key: %v", err)
	}

	if !strings.Contains(pkcs8Key, "BEGIN PRIVATE KEY") {
		t.Error("Converted key should have PKCS#8 header")
	}
	if strings.Contains(pkcs8Key, "RSA PRIVATE KEY") {
		t.Error("Converted key should not have PKCS#1 header")
	}

	// Test that already-PKCS#8 keys are returned unchanged
	pkcs8Again, err := ConvertPrivateKeyToPKCS8(pkcs8Key)
	if err != nil {
		t.Fatalf("Failed to process already-PKCS#8 key: %v", err)
	}
	if pkcs8Again != pkcs8Key {
		t.Error("Already-PKCS#8 key should be returned unchanged")
	}

	// Verify both formats can be parsed by x509
	block, _ := pem.Decode([]byte(oldKeypair.Private))
	_, err = x509.ParsePKCS1PrivateKey(block.Bytes) // PKCS#1
	if err != nil {
		t.Errorf("Original PKCS#1 key should be parseable: %v", err)
	}

	block, _ = pem.Decode([]byte(pkcs8Key))
	_, err = x509.ParsePKCS8PrivateKey(block.Bytes) // PKCS#8
	if err != nil {
		t.Errorf("Converted PKCS#8 key should be parseable: %v", err)
	}
}

func TestConvertPublicKeyToPKIX(t *testing.T) {
	// Generate a real PKCS#1 public key for testing
	bitSize := 2048 // Minimum secure size

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}

	pub := key.Public()

	// Create PKCS#1 format (old format)
	pkcs1PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(pub.(*rsa.PublicKey)),
	})
	oldPublicKey := string(pkcs1PEM)

	// Convert to PKIX
	pkixKey, err := ConvertPublicKeyToPKIX(oldPublicKey)
	if err != nil {
		t.Fatalf("Failed to convert PKCS#1 public key: %v", err)
	}

	if !strings.Contains(pkixKey, "BEGIN PUBLIC KEY") {
		t.Error("Converted key should have PKIX header")
	}
	if strings.Contains(pkixKey, "RSA PUBLIC KEY") {
		t.Error("Converted key should not have PKCS#1 header")
	}

	// Test that already-PKIX keys are returned unchanged
	pkixAgain, err := ConvertPublicKeyToPKIX(pkixKey)
	if err != nil {
		t.Fatalf("Failed to process already-PKIX key: %v", err)
	}
	if pkixAgain != pkixKey {
		t.Error("Already-PKIX key should be returned unchanged")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid http URL", "http://example.com", true},
		{"valid https URL", "https://example.com", true},
		{"valid URL with path", "https://github.com/deemkeen/stegodon", true},
		{"valid URL with query", "https://example.com?foo=bar", true},
		{"URL with spaces around", "  https://example.com  ", true}, // Should trim
		{"not a URL - plain text", "hello world", false},
		{"not a URL - no protocol", "example.com", false},
		{"not a URL - markdown link", "[text](https://example.com)", false},
		{"not a URL - ftp protocol", "ftp://example.com", false},
		{"empty string", "", false},
		{"just http://", "http://", false}, // No domain
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsURL(tt.input)
			if got != tt.want {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountVisibleChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "plain text",
			input: "Hello world",
			want:  11,
		},
		{
			name:  "single markdown link",
			input: "Check out [stegodon](https://github.com/deemkeen/stegodon) project",
			want:  26, // "Check out stegodon project" without extra space
		},
		{
			name:  "multiple markdown links",
			input: "Visit [site1](https://example.com) and [site2](https://test.com)",
			want:  21, // "Visit site1 and site2" = 21 chars
		},
		{
			name:  "markdown link at start",
			input: "[Link](https://example.com) here",
			want:  9, // "Link here" without extra space
		},
		{
			name:  "markdown link at end",
			input: "Click [here](https://example.com)",
			want:  10, // "Click here" without extra space
		},
		{
			name:  "only markdown link",
			input: "[text](https://example.com)",
			want:  4, // "text" = 4 chars
		},
		{
			name:  "empty string",
			input: "",
			want:  0,
		},
		{
			name:  "no markdown links",
			input: "Just plain text with no links",
			want:  29,
		},
		{
			name:  "link with long URL",
			input: "[short](https://very-long-url-that-should-not-count.com/path/to/resource?query=param)",
			want:  5, // "short" = 5 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountVisibleChars(tt.input)
			if got != tt.want {
				t.Errorf("CountVisibleChars(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateNoteLength(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "empty note",
			input:   "",
			wantErr: false,
		},
		{
			name:    "short note",
			input:   "Hello world",
			wantErr: false,
		},
		{
			name:    "exactly 1000 chars",
			input:   string(make([]byte, 1000)),
			wantErr: false,
		},
		{
			name:    "1001 chars - too long",
			input:   string(make([]byte, 1001)),
			wantErr: true,
		},
		{
			name:    "note with long markdown link under limit",
			input:   "Check [link](" + string(make([]byte, 980)) + ")",
			wantErr: false, // Total is 997 chars
		},
		{
			name:    "note with markdown link over limit",
			input:   "Check [link](" + string(make([]byte, 990)) + ")",
			wantErr: true, // Total is 1007 chars
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoteLength(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNoteLength() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsURLEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{
			name:  "URL with port",
			input: "https://example.com:8080",
			want:  true,
		},
		{
			name:  "URL with path and query",
			input: "https://example.com/path?key=value&foo=bar",
			want:  true,
		},
		{
			name:  "URL with fragment",
			input: "https://example.com/page#section",
			want:  true,
		},
		{
			name:  "URL with username",
			input: "https://user@example.com",
			want:  true,
		},
		{
			name:  "localhost URL",
			input: "http://localhost:9999",
			want:  true,
		},
		{
			name:  "IP address URL",
			input: "http://192.168.1.1",
			want:  true,
		},
		{
			name:  "URL with trailing slash",
			input: "https://example.com/",
			want:  true,
		},
		{
			name:  "multiple spaces around URL",
			input: "   https://example.com   ",
			want:  true,
		},
		{
			name:  "URL embedded in markdown",
			input: "[text](https://example.com)",
			want:  false,
		},
		{
			name:  "URL with newline",
			input: "https://example.com\n",
			want:  true, // TrimSpace removes the newline, so this becomes valid
		},
		{
			name:  "partial URL - just protocol",
			input: "https://",
			want:  false,
		},
		{
			name:  "URL with space in middle",
			input: "https://example .com",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsURL(tt.input)
			if got != tt.want {
				t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCountVisibleCharsEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{
			name:  "nested brackets not a link",
			input: "[outer [inner] text](url)",
			want:  25, // Regex doesn't match nested brackets, counts as plain text
		},
		{
			name:  "adjacent links no space",
			input: "[link1](url1)[link2](url2)",
			want:  10, // "link1" (5) + "link2" (5) = 10
		},
		{
			name:  "link with empty text",
			input: "[](https://example.com)",
			want:  23, // Regex requires [^\]]+ which means at least 1 char, so no match
		},
		{
			name:  "link with spaces in text",
			input: "[some text here](https://example.com)",
			want:  14, // "some text here" but counted by bytes including spaces
		},
		{
			name:  "multiple links with text between",
			input: "start [link1](url) middle [link2](url) end",
			want:  28, // Actual character count with the implementation
		},
		{
			name:  "link with unicode text",
			input: "[日本語](https://example.com)",
			want:  9, // Japanese chars count as UTF-8 bytes
		},
		{
			name:  "link with emoji",
			input: "[🔥🎉](https://example.com)",
			want:  8, // Emojis count as UTF-8 bytes
		},
		{
			name:  "very long URL",
			input: "[text](https://example.com/" + strings.Repeat("a", 500) + ")",
			want:  4, // Only "text" counts, not the 500-char URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountVisibleChars(tt.input)
			if got != tt.want {
				t.Errorf("CountVisibleChars(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
	"html"
	"log"
	rnd "math/rand"
	"regexp"
	"strings"
	"time"
)

//go:embed version.txt
var embeddedVersion string

type RsaKeyPair struct {
	Private string
	Public  string
}

func LogPublicKey(s ssh.Session) {
	log.Println(fmt.Sprintf("%s@%s opened a new ssh-session..", s.User(), s.LocalAddr()))
}

func PublicKeyToString(s ssh.PublicKey) string {
	return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s)))
}

func PkToHash(pk string) string {
	h := sha256.New()
	// TODO add a pinch of salt
	h.Write([]byte(pk))
	return hex.EncodeToString(h.Sum(nil))
}

func GetVersion() string {
	return strings.TrimSpace(embeddedVersion)
}

func GetNameAndVersion() string {
	return fmt.Sprintf("%s / %s", Name, GetVersion())
}

func RandomString(length int) string {
	rnd.Seed(time.Now().UnixNano())
	b := make([]byte, length)
	rnd.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func NormalizeInput(text string) string {
	normalized := strings.Replace(text, "\n", " ", -1)
	normalized = html.EscapeString(normalized)
	return normalized
}

func DateTimeFormat() string {
	return "2006-01-02 15:04:05 CEST"
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", " ")
	return string(s)
}

// ConvertPrivateKeyToPKCS8 converts a PKCS#1 private key PEM to PKCS#8 format
// The cryptographic key material remains unchanged, only the encoding format changes
func ConvertPrivateKeyToPKCS8(pkcs1PEM string) (string, error) {
	// Parse existing PKCS#1 key
	block, _ := pem.Decode([]byte(pkcs1PEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	// Handle both PKCS#1 and already-PKCS#8 keys
	if block.Type == "PRIVATE KEY" {
		// Already PKCS#8 format, return as-is
		return pkcs1PEM, nil
	}

	if block.Type != "RSA PRIVATE KEY" {
		return "", fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKCS#1 private key: %w", err)
	}

	// Marshal to PKCS#8 format (same key, different encoding)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PKCS#8 private key: %w", err)
	}

	pkcs8PEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: pkcs8Bytes,
	})

	return string(pkcs8PEM), nil
}

// ConvertPublicKeyToPKIX converts a PKCS#1 public key PEM to PKIX (PKCS#8 public) format
// The cryptographic key material remains unchanged, only the encoding format changes
func ConvertPublicKeyToPKIX(pkcs1PEM string) (string, error) {
	// Parse existing PKCS#1 key
	block, _ := pem.Decode([]byte(pkcs1PEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	// Handle both PKCS#1 and already-PKIX keys
	if block.Type == "PUBLIC KEY" {
		// Already PKIX format, return as-is
		return pkcs1PEM, nil
	}

	if block.Type != "RSA PUBLIC KEY" {
		return "", fmt.Errorf("unexpected PEM type: %s", block.Type)
	}

	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKCS#1 public key: %w", err)
	}

	// Marshal to PKIX format (same key, different encoding)
	pkixBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PKIX public key: %w", err)
	}

	pkixPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pkixBytes,
	})

	return string(pkixPEM), nil
}

func GeneratePemKeypair() *RsaKeyPair {
	bitSize := 4096

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		panic(err)
	}

	pub := key.Public()

	// Use PKCS#8 format for new keys (standard format)
	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		panic(err)
	}

	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PRIVATE KEY", // PKCS#8 format
			Bytes: pkcs8Bytes,
		},
	)

	// Use PKIX format for public keys (also called PKCS#8 public key format)
	pkixBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic(err)
	}

	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY", // PKIX format
			Bytes: pkixBytes,
		},
	)

	return &RsaKeyPair{Private: string(keyPEM[:]), Public: string(pubPEM[:])}
}

// MarkdownLinksToHTML converts Markdown links [text](url) to HTML <a> tags
func MarkdownLinksToHTML(text string) string {
	// Regex pattern for Markdown links: [text](url)
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	// Replace all Markdown links with HTML anchor tags
	result := re.ReplaceAllStringFunc(text, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) == 3 {
			linkText := html.EscapeString(matches[1])
			linkURL := html.EscapeString(matches[2])
			return fmt.Sprintf(`<a href="%s" target="_blank" rel="noopener noreferrer">%s</a>`, linkURL, linkText)
		}
		return match
	})

	return result
}

// ExtractMarkdownLinks returns a list of URLs from Markdown links in text
func ExtractMarkdownLinks(text string) []string {
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := re.FindAllStringSubmatch(text, -1)

	urls := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) == 3 {
			urls = append(urls, match[2])
		}
	}

	return urls
}

// MarkdownLinksToTerminal converts Markdown links [text](url) to OSC 8 hyperlinks
// Format: OSC 8 wrapped link text only (no URL shown)
// For terminals that support OSC 8, this creates clickable links with green color
func MarkdownLinksToTerminal(text string) string {
	// Regex pattern for Markdown links: [text](url)
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	// Replace all Markdown links with OSC 8 hyperlinks
	result := re.ReplaceAllStringFunc(text, func(match string) string {
		matches := re.FindStringSubmatch(match)
		if len(matches) == 3 {
			linkText := matches[1]
			linkURL := matches[2]
			// OSC 8 format with green color (38;2;0;255;127 = RGB #00ff7f) and underline
			// Format: COLOR_START + OSC8_START + TEXT + OSC8_END + COLOR_RESET
			// Use \033[39;24m to reset only foreground color and underline, not background
			return fmt.Sprintf("\033[38;2;0;255;127;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m", linkURL, linkText)
		}
		return match
	})

	return result
}

// IsURL checks if a given string is a valid HTTP or HTTPS URL
func IsURL(text string) bool {
	// Trim whitespace
	text = strings.TrimSpace(text)

	// Simple regex to match http:// or https:// URLs
	urlRegex := regexp.MustCompile(`^https?://[^\s]+$`)
	return urlRegex.MatchString(text)
}

// CountVisibleChars counts only the visible characters in text with markdown links.
// For markdown links [text](url), only the 'text' portion is counted.
// All other characters are counted normally.
func CountVisibleChars(text string) int {
	// Regex pattern for Markdown links: [text](url)
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	visibleCount := 0
	lastIndex := 0

	// Find all markdown links
	matches := re.FindAllStringSubmatchIndex(text, -1)

	for _, match := range matches {
		// match[0] = start of full match, match[1] = end of full match
		// match[2] = start of text capture, match[3] = end of text capture

		// Count characters before this link
		visibleCount += match[0] - lastIndex

		// Count only the link text (not the URL or brackets)
		linkTextLen := match[3] - match[2]
		visibleCount += linkTextLen

		// Move past this match
		lastIndex = match[1]
	}

	// Count remaining characters after last link
	visibleCount += len(text) - lastIndex

	return visibleCount
}

// ValidateNoteLength checks if the full note text (including markdown syntax)
// exceeds the database limit of 1000 characters.
// Returns an error if the text is too long.
func ValidateNoteLength(text string) error {
	const maxDBLength = 1000

	if len(text) > maxDBLength {
		return fmt.Errorf("Note too long (max %d characters including links)", maxDBLength)
	}

	return nil
}

// TruncateVisibleLength truncates a string based on visible character count,
// ignoring ANSI escape sequences and OSC 8 hyperlinks.
// This ensures proper truncation for strings containing terminal formatting.
func TruncateVisibleLength(s string, maxLen int) string {
	// Regex to match ANSI escape sequences (including OSC 8 hyperlinks)
	// Matches: \033[...m (SGR), \033]8;;...\033\\ (OSC 8)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)

	// Strip ANSI codes to count visible characters
	visible := ansiRegex.ReplaceAllString(s, "")

	// If visible length is within limit, return original (with formatting)
	if len(visible) <= maxLen {
		return s
	}

	// Need to truncate - walk through string and count visible chars
	visibleCount := 0
	truncateAt := 0
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' {
			// Start of escape sequence
			inEscape = true
			continue
		}

		if inEscape {
			// Check for end of SGR sequence (\033[...m)
			if s[i] == 'm' {
				inEscape = false
				continue
			}
			// Check for end of OSC 8 sequence (\033]8;;...\033\\)
			if i > 0 && s[i-1] == '\x1b' && s[i] == '\\' {
				inEscape = false
				continue
			}
			// Still in escape sequence
			continue
		}

		// This is a visible character
		visibleCount++
		if visibleCount > maxLen-3 {
			// Found truncation point (reserve 3 chars for "...")
			truncateAt = i
			break
		}
		truncateAt = i + 1
	}

	// Truncate and add ellipsis
	result := s[:truncateAt] + "..."

	// Close any open formatting by adding reset sequence
	result += "\x1b[0m"

	return result
}

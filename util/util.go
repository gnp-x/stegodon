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

func GeneratePemKeypair() *RsaKeyPair {
	bitSize := 4096

	key, err := rsa.GenerateKey(rand.Reader, bitSize)
	if err != nil {
		panic(err)
	}

	pub := key.Public()

	keyPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		},
	)

	pubPEM := pem.EncodeToMemory(
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(pub.(*rsa.PublicKey)),
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

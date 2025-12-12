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
	"html"
	"log"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/ssh"
	gossh "golang.org/x/crypto/ssh"
)

//go:embed version.txt
var embeddedVersion string

// Pre-compiled regex patterns for performance
var ansiEscapeRegex = regexp.MustCompile(`\x1b\[[0-9;]*m|\x1b\]8;;[^\x1b]*\x1b\\`)
var hashtagRegex = regexp.MustCompile(`#([a-zA-Z][a-zA-Z0-9_]*)`)
var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9_]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
var markdownLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// ANSI color codes for terminal highlighting (must match ui/common/styles.go values)
const (
	ansiHashtagColor = "75"        // ANSI 75 (#5fafff) - matches COLOR_HASHTAG
	ansiMentionColor = "48"        // ANSI 48 (#00ff87) - matches COLOR_MENTION
	ansiLinkRGB      = "0;255;135" // RGB for links (#00ff87) - matches COLOR_LINK_RGB
)

var urlRegex = regexp.MustCompile(`^https?://[^\s]+$`)

type RsaKeyPair struct {
	Private string
	Public  string
}

func LogPublicKey(s ssh.Session) {
	log.Printf("%s@%s opened a new ssh-session..", s.User(), s.LocalAddr())
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
	b := make([]byte, length)
	rand.Read(b)
	return fmt.Sprintf("%x", b)[:length]
}

func NormalizeInput(text string) string {
	normalized := strings.ReplaceAll(text, "\n", " ")
	normalized = html.EscapeString(normalized)
	return normalized
}

// StripHTMLTags removes HTML tags from a string and converts common HTML entities
func StripHTMLTags(html string) string {
	// Remove all HTML tags using pre-compiled regex
	text := htmlTagRegex.ReplaceAllString(html, "")

	// Convert common HTML entities
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "\\n", "\n")
	text = strings.ReplaceAll(text, "\\\"", "\"")

	// Clean up extra whitespace
	text = strings.TrimSpace(text)

	return text
}

func DateTimeFormat() string {
	return "2006-01-02 15:04:05 CEST"
}

func PrettyPrint(i any) string {
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
	// Replace all Markdown links with HTML anchor tags
	result := markdownLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
		matches := markdownLinkRegex.FindStringSubmatch(match)
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
	matches := markdownLinkRegex.FindAllStringSubmatch(text, -1)

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
// For terminals that support OSC 8, this creates clickable links with link color
func MarkdownLinksToTerminal(text string) string {
	// Replace all Markdown links with OSC 8 hyperlinks
	result := markdownLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
		matches := markdownLinkRegex.FindStringSubmatch(match)
		if len(matches) == 3 {
			linkText := matches[1]
			linkURL := matches[2]
			// OSC 8 format with link color (RGB) and underline
			// Format: COLOR_START + OSC8_START + TEXT + OSC8_END + COLOR_RESET
			// Use \033[39;24m to reset only foreground color and underline, not background
			return fmt.Sprintf("\033[38;2;"+ansiLinkRGB+";4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m", linkURL, linkText)
		}
		return match
	})

	return result
}

// GetMarkdownLinkCount returns the number of valid markdown links in the text
func GetMarkdownLinkCount(text string) int {
	return len(markdownLinkRegex.FindAllString(text, -1))
}

// IsURL checks if a given string is a valid HTTP or HTTPS URL
func IsURL(text string) bool {
	// Trim whitespace
	text = strings.TrimSpace(text)

	return urlRegex.MatchString(text)
}

// CountVisibleChars counts only the visible characters (runes) in text, ignoring:
// - Markdown links [text](url) - only the 'text' portion is counted
// - ANSI escape sequences (SGR codes like \033[38;5;75m)
// - OSC 8 hyperlinks (\033]8;;url\033\\text\033]8;;\033\\)
// This function counts Unicode runes, not bytes, so multi-byte characters like
// "·" (middle dot) are counted as 1 visible character.
func CountVisibleChars(text string) int {
	// First, strip all ANSI escape sequences (SGR and OSC 8)
	stripped := ansiEscapeRegex.ReplaceAllString(text, "")

	// Find all markdown links and replace them with just the link text
	// This way we can simply count runes on the final string
	result := markdownLinkRegex.ReplaceAllString(stripped, "$1")

	return utf8.RuneCountInString(result)
}

// ValidateNoteLength checks if the full note text (including markdown syntax)
// exceeds the database limit.
// Returns an error if the text is too long.
func ValidateNoteLength(text string) error {
	const maxDBLength = 1000 // Must match common.MaxNoteDBLength

	if len(text) > maxDBLength {
		return fmt.Errorf("Note too long (max %d characters including links)", maxDBLength)
	}

	return nil
}

// TruncateVisibleLength truncates a string based on visible character count,
// ignoring ANSI escape sequences and OSC 8 hyperlinks.
// This ensures proper truncation for strings containing terminal formatting.
func TruncateVisibleLength(s string, maxLen int) string {
	// Use pre-compiled regex for performance (was being compiled on every call)
	// Strip ANSI codes to count visible characters
	visible := ansiEscapeRegex.ReplaceAllString(s, "")

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

// ParseHashtags extracts hashtags from text and returns them as lowercase, deduplicated strings.
// Hashtags must start with a letter and can contain letters, numbers, and underscores.
// Examples: #hello, #Go123, #my_tag are valid; #123, #_ are not.
func ParseHashtags(text string) []string {
	matches := hashtagRegex.FindAllStringSubmatch(text, -1)

	// Use map for deduplication (lowercase)
	seen := make(map[string]bool)
	tags := make([]string, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 2 {
			tag := strings.ToLower(match[1])
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}

	return tags
}

// HighlightHashtagsTerminal colors hashtags in text for terminal display.
// Uses secondary color for hashtags to make them visually distinct.
func HighlightHashtagsTerminal(text string) string {
	// Use the same regex pattern as hashtagRegex
	return hashtagRegex.ReplaceAllString(text, "\033[38;5;"+ansiHashtagColor+"m#$1\033[39m")
}

// HighlightHashtagsHTML converts hashtags in text to clickable HTML links.
// Each hashtag becomes a link to /tags/{tag} page.
func HighlightHashtagsHTML(text string) string {
	return hashtagRegex.ReplaceAllString(text, `<a href="/tags/$1" class="hashtag">#$1</a>`)
}

// HashtagsToActivityPubHTML converts hashtags in text to ActivityPub-compliant HTML links.
// Uses the format: <a href="https://hostname/tags/tag" class="hashtag" rel="tag">#<span>tag</span></a>
// The baseURL should be the full https:// URL of the server (e.g., "https://example.com")
func HashtagsToActivityPubHTML(text string, baseURL string) string {
	return hashtagRegex.ReplaceAllStringFunc(text, func(match string) string {
		// match is the full hashtag including # (e.g., "#something")
		submatches := hashtagRegex.FindStringSubmatch(match)
		if len(submatches) >= 2 {
			tag := strings.ToLower(submatches[1])
			return fmt.Sprintf(`<a href="%s/tags/%s" class="hashtag" rel="tag">#<span>%s</span></a>`, baseURL, tag, tag)
		}
		return match
	})
}

// Mention represents a parsed @username@domain mention
type Mention struct {
	Username string
	Domain   string
}

// ParseMentions extracts @username@domain mentions from text.
// Returns deduplicated mentions preserving order of first occurrence.
func ParseMentions(text string) []Mention {
	matches := mentionRegex.FindAllStringSubmatch(text, -1)

	// Use map for deduplication (lowercase key)
	seen := make(map[string]bool)
	mentions := make([]Mention, 0, len(matches))

	for _, match := range matches {
		if len(match) >= 3 {
			username := strings.ToLower(match[1])
			domain := strings.ToLower(match[2])
			key := username + "@" + domain
			if !seen[key] {
				seen[key] = true
				mentions = append(mentions, Mention{
					Username: username,
					Domain:   domain,
				})
			}
		}
	}

	return mentions
}

// HighlightMentionsTerminal colors mentions in text for terminal display and makes them clickable.
// Uses mention color with OSC 8 hyperlinks to the user's profile.
// Local users are displayed as @username, remote users as @username@domain.
func HighlightMentionsTerminal(text string, localDomain string) string {
	return mentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionRegex.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			username := submatches[1]
			domain := submatches[2]

			var displayMention string
			var profileURL string

			if strings.EqualFold(domain, localDomain) {
				// Local user - show just @username, link to local profile
				displayMention = fmt.Sprintf("@%s", username)
				profileURL = fmt.Sprintf("https://%s/u/%s", localDomain, username)
			} else {
				// Remote user - show @username@domain, link to their instance
				displayMention = fmt.Sprintf("@%s@%s", username, domain)
				profileURL = fmt.Sprintf("https://%s/@%s", domain, username)
			}

			// OSC 8 format with mention color and underline
			// Format: COLOR_START + OSC8_START + TEXT + OSC8_END + COLOR_RESET
			return fmt.Sprintf("\033[38;5;"+ansiMentionColor+";4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m", profileURL, displayMention)
		}
		return match
	})
}

// HighlightMentionsHTML converts mentions in text to clickable HTML links.
// For local users (same domain), displays as @username and links to /users/{username}.
// For remote users, displays as @username@domain and links to their profile URL.
func HighlightMentionsHTML(text string, localDomain string) string {
	return mentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionRegex.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			username := submatches[1]
			domain := submatches[2]
			if strings.EqualFold(domain, localDomain) {
				// Local user - show just @username, link to local profile
				return fmt.Sprintf(`<a href="/u/%s" class="mention">@%s</a>`, username, username)
			}
			// Remote user - show @username@domain, link to their instance profile
			return fmt.Sprintf(`<a href="https://%s/@%s" class="mention" target="_blank" rel="noopener noreferrer">@%s@%s</a>`, domain, username, username, domain)
		}
		return match
	})
}

// MentionsToActivityPubHTML converts mentions in text to ActivityPub-compliant HTML links.
// Uses the format: <span class="h-card"><a href="actorURI" class="u-url mention">@<span>username</span></a></span>
// This requires pre-resolved actor URIs, so it takes a map of mention -> actorURI.
func MentionsToActivityPubHTML(text string, mentionURIs map[string]string) string {
	return mentionRegex.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mentionRegex.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			username := strings.ToLower(submatches[1])
			domain := strings.ToLower(submatches[2])
			key := "@" + username + "@" + domain
			if actorURI, ok := mentionURIs[key]; ok {
				return fmt.Sprintf(`<span class="h-card"><a href="%s" class="u-url mention">@<span>%s</span></a></span>`, actorURI, username)
			}
			// Fallback if URI not found - just link to profile
			return fmt.Sprintf(`<span class="h-card"><a href="https://%s/@%s" class="u-url mention">@<span>%s</span></a></span>`, domain, username, username)
		}
		return match
	})
}

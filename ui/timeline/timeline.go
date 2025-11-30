package timeline

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_GREEN)).
			Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY)).
			Italic(true)

	// Inverted styles for selected posts
	selectedTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)) // White

	selectedAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)). // White
				Bold(true)

	selectedContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)) // White
)

type Model struct {
	AccountId  uuid.UUID
	Posts      []FederatedPost
	Offset     int  // Pagination offset
	Selected   int  // Currently selected post index
	Width      int
	Height     int
	isActive   bool // Track if this view is currently visible (prevents ticker leaks)
	showingURL bool // Track if URL is displayed instead of content for selected post
}

type FederatedPost struct {
	Actor     string
	Content   string
	Time      time.Time
	ObjectURI string // URL to the original post
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId:  accountId,
		Posts:      []FederatedPost{},
		Offset:     0,
		Selected:   0,
		Width:      width,
		Height:     height,
		isActive:   false, // Start inactive, will be activated when view is shown
		showingURL: false, // Start in content mode
	}
}

func (m Model) Init() tea.Cmd {
	// Don't start any commands here - model starts inactive
	// ActivateViewMsg handler will load data and start ticker when view becomes active
	// This prevents tea.Batch() from spawning goroutines that accumulate
	return nil
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg every 10 seconds
func tickRefresh() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.DeactivateViewMsg:
		// View is becoming inactive (user navigated away)
		m.isActive = false
		return m, nil

	case common.ActivateViewMsg:
		// View is becoming active (user navigated here)
		m.isActive = true
		// Load data first, tick will be scheduled when data arrives
		return m, loadFederatedPosts(m.AccountId)

	case refreshTickMsg:
		// Only schedule next refresh if view is still active
		if m.isActive {
			return m, loadFederatedPosts(m.AccountId) // Just load data, tick scheduled after load
		}
		// View is inactive, stop the ticker chain
		return m, nil

	case postsLoadedMsg:
		m.Posts = msg.posts
		// Keep selection within bounds after reload
		if m.Selected >= len(m.Posts) {
			m.Selected = max(0, len(m.Posts)-1)
		}
		// Keep Offset in sync
		m.Offset = m.Selected

		// Schedule next tick AFTER data loads (only if still active)
		// This prevents tea.Batch() goroutine accumulation
		if m.isActive {
			return m, tickRefresh()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected // Keep selected at top
			}
			m.showingURL = false // Reset to content when navigating
		case "down", "j":
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected // Keep selected at top
			}
			m.showingURL = false // Reset to content when navigating
		case "o":
			// Toggle between showing content and URL
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				if selectedPost.ObjectURI != "" {
					m.showingURL = !m.showingURL
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("federated timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No federated posts yet.\nFollow some accounts to see their posts here!"))
	} else {
		// Calculate right panel width for selection background
		leftPanelWidth := m.Width / 3
		rightPanelWidth := m.Width - leftPanelWidth - 6

		itemsPerPage := 5
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Format timestamp
			timeStr := formatTime(post.Time)

			// Apply selection highlighting - full width box with inverted colors
			if i == m.Selected {
				// Use consistent width for all lines
				contentMaxWidth := min(150, rightPanelWidth-4)

				// Truncate and pad time to fit within max width
				truncatedTime := util.TruncateVisibleLength(timeStr, contentMaxWidth)
				timeVisibleLen := util.CountVisibleChars(truncatedTime)
				timePaddingNeeded := contentMaxWidth - timeVisibleLen
				if timePaddingNeeded < 0 {
					timePaddingNeeded = 0
				}
				paddedTime := truncatedTime + strings.Repeat(" ", timePaddingNeeded)

				// Truncate and pad author to fit within max width
				truncatedAuthor := util.TruncateVisibleLength(post.Actor, contentMaxWidth)
				authorVisibleLen := util.CountVisibleChars(truncatedAuthor)
				authorPaddingNeeded := contentMaxWidth - authorVisibleLen
				if authorPaddingNeeded < 0 {
					authorPaddingNeeded = 0
				}
				paddedAuthor := truncatedAuthor + strings.Repeat(" ", authorPaddingNeeded)

				// Apply background style without Width (we handle padding manually)
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_LIGHTBLUE))

				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(paddedTime))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(paddedAuthor))

				// Toggle between content and URL
				if m.showingURL && post.ObjectURI != "" {
					linkText := "🔗 " + post.ObjectURI

					// Truncate the display text to fit
					truncatedLinkText := util.TruncateVisibleLength(linkText, contentMaxWidth)

					// Create OSC 8 clickable hyperlink with visual indicator
					osc8Link := fmt.Sprintf("\033[38;2;0;255;127;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m",
						post.ObjectURI, truncatedLinkText) // Still link to full URL

					// Calculate visible length for padding to match panel width
					visibleLen := util.CountVisibleChars(truncatedLinkText)
					paddingNeeded := contentMaxWidth - visibleLen
					if paddingNeeded < 0 {
						paddingNeeded = 0
					}
					paddedLink := osc8Link + strings.Repeat(" ", paddingNeeded)

					// Render hint text with padding
					hintText := "(Cmd+click to open, press 'o' to toggle back)"
					hintVisibleLen := len(hintText)
					hintPaddingNeeded := contentMaxWidth - hintVisibleLen
					if hintPaddingNeeded < 0 {
						hintPaddingNeeded = 0
					}
					paddedHint := hintText + strings.Repeat(" ", hintPaddingNeeded)

					// Create blank lines with padding
					blankLine := strings.Repeat(" ", contentMaxWidth)

					// Combine all content lines
					combinedContent := paddedLink + "\n" + blankLine + "\n" + paddedHint

					// Apply styles to the combined content
					contentStyle := lipgloss.NewStyle().
						Background(lipgloss.Color(common.COLOR_LIGHTBLUE)).
						Foreground(lipgloss.Color(common.COLOR_WHITE))
					contentFormatted := contentStyle.Render(combinedContent)

					s.WriteString(timeFormatted + "\n")
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				} else {
					// For regular content, truncate and pad manually
					truncatedContent := util.TruncateVisibleLength(post.Content, contentMaxWidth)
					contentVisibleLen := util.CountVisibleChars(truncatedContent)
					contentPaddingNeeded := contentMaxWidth - contentVisibleLen
					if contentPaddingNeeded < 0 {
						contentPaddingNeeded = 0
					}
					paddedContent := truncatedContent + strings.Repeat(" ", contentPaddingNeeded)

					contentFormatted := selectedBg.Render(selectedContentStyle.Render(paddedContent))
					s.WriteString(timeFormatted + "\n")
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				}
			} else {
				// Apply same width to unselected items for consistent wrapping
				unselectedStyle := lipgloss.NewStyle().
					Width(rightPanelWidth - 4)

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr))
				authorFormatted := unselectedStyle.Render(authorStyle.Render(post.Actor))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(util.TruncateVisibleLength(post.Content, 150)))

				s.WriteString(timeFormatted + "\n")
				s.WriteString(authorFormatted + "\n")
				s.WriteString(contentFormatted)
			}

			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []FederatedPost
}

// loadFederatedPosts loads recent federated activities from followed remote users
func loadFederatedPosts(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, activities := database.ReadFederatedActivities(accountId, 50) // Increased from 20 to 50
		if err != nil {
			log.Printf("Failed to load federated activities: %v", err)
			return postsLoadedMsg{posts: []FederatedPost{}}
		}

		if activities == nil {
			return postsLoadedMsg{posts: []FederatedPost{}}
		}

		// Parse activities into posts
		posts := make([]FederatedPost, 0, len(*activities))
		for _, activity := range *activities {
			// Parse the raw JSON to extract content
			// Handle both Create and Update activities (Update is stored in Create activities)
			var activityWrapper struct {
				Type   string `json:"type"`
				Object struct {
					ID      string `json:"id"`
					Content string `json:"content"`
				} `json:"object"`
			}

			if err := json.Unmarshal([]byte(activity.RawJSON), &activityWrapper); err != nil {
				log.Printf("Failed to parse activity JSON: %v", err)
				continue
			}

			// Skip if content is empty
			if activityWrapper.Object.Content == "" {
				continue
			}

			// Strip HTML tags from content
			cleanContent := stripHTMLTags(activityWrapper.Object.Content)

			// Get remote account to format handle as username@domain
			handle := activity.ActorURI // fallback to URI
			err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
			if err == nil && remoteAcc != nil {
				handle = "@" + remoteAcc.Username + "@" + remoteAcc.Domain
			}

			// Use ObjectURI from activity, or extract from raw JSON if empty
			objectURI := activity.ObjectURI
			if objectURI == "" && activityWrapper.Object.ID != "" {
				objectURI = activityWrapper.Object.ID
			}

			posts = append(posts, FederatedPost{
				Actor:     handle,
				Content:   cleanContent,
				Time:      activity.CreatedAt,
				ObjectURI: objectURI,
			})
		}

		return postsLoadedMsg{posts: posts}
	}
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

// stripHTMLTags removes HTML tags from a string and converts common HTML entities
func stripHTMLTags(html string) string {
	// Remove all HTML tags
	text := htmlTagRegex.ReplaceAllString(html, "")

	// Convert common HTML entities
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Clean up extra whitespace
	text = strings.TrimSpace(text)

	return text
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

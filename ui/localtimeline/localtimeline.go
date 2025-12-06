package localtimeline

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
	"log"
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

	// Selected post styles (inverted colors)
	selectedTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE)).
				Bold(true)

	selectedContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(common.COLOR_LIGHTBLUE))
)

type Model struct {
	AccountId uuid.UUID
	Posts     []domain.Note
	Offset    int  // Pagination offset
	Selected  int  // Currently selected post index
	Width     int
	Height    int
	isActive  bool // Track if this view is currently visible (prevents ticker leaks)
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId: accountId,
		Posts:     []domain.Note{},
		Offset:    0,
		Selected:  0,
		Width:     width,
		Height:    height,
		isActive:  false, // Start inactive, will be activated when view is shown
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
		return m, loadLocalPosts(m.AccountId)

	case refreshTickMsg:
		// Only schedule next refresh if view is still active
		if m.isActive {
			return m, loadLocalPosts(m.AccountId) // Just load data, tick scheduled after load
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
		case "down", "j":
			// Allow scrolling if we have any posts at all
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected // Keep selected at top
			}
		case "r":
			// Reply to selected local post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// For local posts, use the ObjectURI if set, otherwise construct from note ID
				objectURI := selectedPost.ObjectURI
				if objectURI == "" && selectedPost.Id != uuid.Nil {
					// Local notes use their UUID-based URI
					conf, err := util.ReadConf()
					if err == nil && conf.Conf.SslDomain != "" {
						objectURI = fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, selectedPost.Id.String())
					}
				}
				// Create preview from content (first line or truncated)
				preview := selectedPost.Message
				if idx := strings.Index(preview, "\n"); idx > 0 {
					preview = preview[:idx]
				}
				return m, func() tea.Msg {
					return common.ReplyToNoteMsg{
						NoteURI: objectURI,
						Author:  selectedPost.CreatedBy,
						Preview: preview,
					}
				}
			}
		case "enter":
			// Open thread view for selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				objectURI := selectedPost.ObjectURI
				if objectURI == "" && selectedPost.Id != uuid.Nil {
					conf, err := util.ReadConf()
					if err == nil && conf.Conf.SslDomain != "" {
						objectURI = fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, selectedPost.Id.String())
					}
				}
				return m, func() tea.Msg {
					return common.ViewThreadMsg{
						NoteURI:   objectURI,
						NoteID:    selectedPost.Id,
						IsLocal:   true,
						Author:    selectedPost.CreatedBy,
						Content:   selectedPost.Message,
						CreatedAt: selectedPost.CreatedAt,
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("local timeline (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No local posts yet.\nCreate some notes or invite others to join!"))
	} else {
		itemsPerPage := common.DefaultItemsPerPage
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Convert Markdown links to OSC 8 hyperlinks and highlight hashtags
			messageWithLinks := util.MarkdownLinksToTerminal(post.Message)
			messageWithLinksAndHashtags := util.HighlightHashtagsTerminal(messageWithLinks)

			// Render in vertical layout like notes list
			var timeStr, authorStr, contentStr string

			if i == m.Selected {
				// Selected post: use inverted colors
				timeStr = selectedBgStyle.Render(selectedTimeStyle.Render(formatTime(post.CreatedAt)))
				authorStr = selectedBgStyle.Render(selectedAuthorStyle.Render("@" + post.CreatedBy))
				contentStr = selectedBgStyle.Render(selectedContentStyle.Render(util.TruncateVisibleLength(messageWithLinksAndHashtags, 150)))
			} else {
				// Normal post
				timeStr = timeStyle.Render(formatTime(post.CreatedAt))
				authorStr = authorStyle.Render("@" + post.CreatedBy)
				contentStr = contentStyle.Render(util.TruncateVisibleLength(messageWithLinksAndHashtags, 150))
			}

			postContent := lipgloss.JoinVertical(lipgloss.Left, timeStr, authorStr, contentStr)
			s.WriteString(postContent)
			s.WriteString("\n\n")
		}
	}

	return s.String()
}

// postsLoadedMsg is sent when posts are loaded
type postsLoadedMsg struct {
	posts []domain.Note
}

// loadLocalPosts loads recent posts from followed local users (plus own posts)
func loadLocalPosts(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, notes := database.ReadLocalTimelineNotes(accountId, 50)
		if err != nil {
			log.Printf("Failed to load local timeline: %v", err)
			return postsLoadedMsg{posts: []domain.Note{}}
		}

		if notes == nil {
			return postsLoadedMsg{posts: []domain.Note{}}
		}

		return postsLoadedMsg{posts: *notes}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
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

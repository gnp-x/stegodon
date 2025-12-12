package hometimeline

import (
	"fmt"
	"log"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

var (
	timeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DIM))

	authorStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_USERNAME)).
			Bold(true)

	// Remote author uses secondary color to differentiate from local
	remoteAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_SECONDARY)).
				Bold(true)

	contentStyle = lipgloss.NewStyle().
			Align(lipgloss.Left)

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DIM)).
			Italic(true)

	// Inverted styles for selected posts
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
)

type Model struct {
	AccountId  uuid.UUID
	Posts      []domain.HomePost
	Offset     int // Pagination offset
	Selected   int // Currently selected post index
	Width      int
	Height     int
	isActive   bool // Track if this view is currently visible (prevents ticker leaks)
	showingURL bool // Track if URL is displayed instead of content for selected post
}

func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId:  accountId,
		Posts:      []domain.HomePost{},
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
	return nil
}

// refreshTickMsg is sent periodically to refresh the timeline
type refreshTickMsg struct{}

// tickRefresh returns a command that sends refreshTickMsg every TimelineRefreshSeconds
func tickRefresh() tea.Cmd {
	return tea.Tick(common.TimelineRefreshSeconds*time.Second, func(t time.Time) tea.Msg {
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
		// Reset scroll position to top when switching to this view
		m.Selected = 0
		m.Offset = 0
		m.showingURL = false
		// Load data first, tick will be scheduled when data arrives
		return m, loadHomePosts(m.AccountId)

	case common.SessionState:
		// Handle UpdateNoteList to refresh when notes are created/updated
		// Always reload data when notes change, regardless of active state
		// The isActive flag only controls the ticker chain, not one-time reloads
		if msg == common.UpdateNoteList {
			return m, loadHomePosts(m.AccountId)
		}
		return m, nil

	case refreshTickMsg:
		// Only schedule next refresh if view is still active
		if m.isActive {
			return m, loadHomePosts(m.AccountId)
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
		if m.isActive {
			return m, tickRefresh()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > 0 {
				m.Selected--
				m.Offset = m.Selected
			}
			m.showingURL = false
		case "down", "j":
			if len(m.Posts) > 0 && m.Selected < len(m.Posts)-1 {
				m.Selected++
				m.Offset = m.Selected
			}
			m.showingURL = false
		case "o":
			// Toggle between showing content and URL (only for posts with ObjectURI)
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				if selectedPost.ObjectURI != "" {
					m.showingURL = !m.showingURL
				}
			}
		case "r":
			// Reply to selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				replyURI := selectedPost.ObjectURI

				// For local posts without object_uri, we can still reply using the note ID
				// The writenote component will handle constructing the proper URI
				if replyURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					// Use a placeholder URI that writenote can resolve
					replyURI = "local:" + selectedPost.NoteID.String()
				}

				if replyURI != "" {
					preview := selectedPost.Content
					if idx := strings.Index(preview, "\n"); idx > 0 {
						preview = preview[:idx]
					}
					return m, func() tea.Msg {
						return common.ReplyToNoteMsg{
							NoteURI: replyURI,
							Author:  selectedPost.Author,
							Preview: preview,
						}
					}
				}
			}
		case "enter":
			// Open thread view for selected post (only if it has replies)
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				// Skip if no replies
				if selectedPost.ReplyCount == 0 {
					return m, nil
				}
				// For local posts, we can use NoteID even without ObjectURI
				if selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					noteURI := selectedPost.ObjectURI
					if noteURI == "" {
						// Use local: prefix for local notes without ObjectURI
						noteURI = "local:" + selectedPost.NoteID.String()
					}
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   noteURI,
							NoteID:    selectedPost.NoteID,
							Author:    selectedPost.Author,
							Content:   selectedPost.Content,
							CreatedAt: selectedPost.Time,
							IsLocal:   true,
						}
					}
				} else if selectedPost.ObjectURI != "" {
					// Remote posts must have ObjectURI
					return m, func() tea.Msg {
						return common.ViewThreadMsg{
							NoteURI:   selectedPost.ObjectURI,
							NoteID:    selectedPost.NoteID,
							Author:    selectedPost.Author,
							Content:   selectedPost.Content,
							CreatedAt: selectedPost.Time,
							IsLocal:   selectedPost.IsLocal,
						}
					}
				}
			}
		case "l":
			// Like/unlike the selected post
			if len(m.Posts) > 0 && m.Selected < len(m.Posts) {
				selectedPost := m.Posts[m.Selected]
				noteURI := selectedPost.ObjectURI
				// For local posts without ObjectURI, use local: prefix
				if noteURI == "" && selectedPost.IsLocal && selectedPost.NoteID != uuid.Nil {
					noteURI = "local:" + selectedPost.NoteID.String()
				}
				if noteURI != "" || selectedPost.NoteID != uuid.Nil {
					return m, func() tea.Msg {
						return common.LikeNoteMsg{
							NoteURI: noteURI,
							NoteID:  selectedPost.NoteID,
							IsLocal: selectedPost.IsLocal,
						}
					}
				}
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("home (%d posts)", len(m.Posts))))
	s.WriteString("\n\n")

	if len(m.Posts) == 0 {
		s.WriteString(emptyStyle.Render("No posts yet.\nFollow some accounts to see their posts here!"))
	} else {
		// Calculate right panel width using layout helpers
		leftPanelWidth := common.CalculateLeftPanelWidth(m.Width)
		rightPanelWidth := common.CalculateRightPanelWidth(m.Width, leftPanelWidth)
		contentWidth := common.CalculateContentWidth(rightPanelWidth, 2)

		itemsPerPage := common.DefaultItemsPerPage
		start := m.Offset
		end := start + itemsPerPage
		if end > len(m.Posts) {
			end = len(m.Posts)
		}

		for i := start; i < end; i++ {
			post := m.Posts[i]

			// Format timestamp with engagement indicators
			timeStr := formatTime(post.Time)
			if post.ReplyCount == 1 {
				timeStr = fmt.Sprintf("%s · 1 reply", timeStr)
			} else if post.ReplyCount > 1 {
				timeStr = fmt.Sprintf("%s · %d replies", timeStr, post.ReplyCount)
			}
			if post.LikeCount > 0 {
				timeStr = fmt.Sprintf("%s · ⭐ %d", timeStr, post.LikeCount)
			}
			if post.BoostCount > 0 {
				timeStr = fmt.Sprintf("%s · 🔁 %d", timeStr, post.BoostCount)
			}

			// Format author with @ prefix for all users
			author := post.Author
			if !strings.HasPrefix(author, "@") {
				author = "@" + author
			}

			// Apply selection highlighting
			if i == m.Selected {
				// Create a style that fills the full width (same approach as myposts)
				selectedBg := lipgloss.NewStyle().
					Background(lipgloss.Color(common.COLOR_ACCENT)).
					Width(contentWidth)

				timeFormatted := selectedBg.Render(selectedTimeStyle.Render(timeStr))
				authorFormatted := selectedBg.Render(selectedAuthorStyle.Render(author))

				// Toggle between content and URL
				if m.showingURL && post.ObjectURI != "" {
					linkText := "🔗 " + post.ObjectURI

					truncatedLinkText := util.TruncateVisibleLength(linkText, common.MaxContentTruncateWidth)
					osc8Link := fmt.Sprintf("\033[38;2;%s;4m\033]8;;%s\033\\%s\033]8;;\033\\\033[39;24m",
						common.COLOR_LINK_RGB, post.ObjectURI, truncatedLinkText)

					hintText := "(Cmd+click to open, press 'o' to toggle back)"

					contentStyleBg := lipgloss.NewStyle().
						Background(lipgloss.Color(common.COLOR_ACCENT)).
						Foreground(lipgloss.Color(common.COLOR_WHITE)).
						Width(contentWidth)
					contentFormatted := contentStyleBg.Render(osc8Link + "\n\n" + hintText)

					s.WriteString(timeFormatted + "\n")
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				} else {
					// Convert Markdown links first, then highlight hashtags (same order as myposts)
					processedContent := post.Content
					if post.IsLocal {
						processedContent = util.MarkdownLinksToTerminal(processedContent)
					}
					highlightedContent := util.HighlightHashtagsTerminal(processedContent)
					localDomain := ""
					if conf, err := util.ReadConf(); err == nil {
						localDomain = conf.Conf.SslDomain
					}
					highlightedContent = util.HighlightMentionsTerminal(highlightedContent, localDomain)

					contentFormatted := selectedBg.Render(selectedContentStyle.Render(util.TruncateVisibleLength(highlightedContent, common.MaxContentTruncateWidth)))
					s.WriteString(timeFormatted + "\n")
					s.WriteString(authorFormatted + "\n")
					s.WriteString(contentFormatted)
				}
			} else {
				unselectedStyle := lipgloss.NewStyle().
					Width(contentWidth)

				// Convert Markdown links first, then highlight hashtags (same order as myposts)
				processedContent := post.Content
				if post.IsLocal {
					processedContent = util.MarkdownLinksToTerminal(processedContent)
				}
				highlightedContent := util.HighlightHashtagsTerminal(processedContent)
				localDomain := ""
				if conf, err := util.ReadConf(); err == nil {
					localDomain = conf.Conf.SslDomain
				}
				highlightedContent = util.HighlightMentionsTerminal(highlightedContent, localDomain)

				// Use different author color for local vs remote
				var authorFormatted string
				if post.IsLocal {
					authorFormatted = unselectedStyle.Render(authorStyle.Render(author))
				} else {
					authorFormatted = unselectedStyle.Render(remoteAuthorStyle.Render(author))
				}

				timeFormatted := unselectedStyle.Render(timeStyle.Render(timeStr))
				contentFormatted := unselectedStyle.Render(contentStyle.Render(util.TruncateVisibleLength(highlightedContent, common.MaxContentTruncateWidth)))

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
	posts []domain.HomePost
}

// loadHomePosts loads the unified home timeline
func loadHomePosts(accountId uuid.UUID) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()
		err, posts := database.ReadHomeTimelinePosts(accountId, common.HomeTimelinePostLimit)
		if err != nil {
			log.Printf("Failed to load home timeline: %v", err)
			return postsLoadedMsg{posts: []domain.HomePost{}}
		}

		if posts == nil {
			return postsLoadedMsg{posts: []domain.HomePost{}}
		}

		return postsLoadedMsg{posts: *posts}
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

func formatTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		mins := int(duration.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if duration < common.HoursPerDay*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(duration.Hours() / common.HoursPerDay)
		return fmt.Sprintf("%dd ago", days)
	}
}

package threadview

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
	// Parent post styles (highlighted)
	parentTimeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY))

	parentAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_GREEN)).
				Bold(true)

	parentContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left)

	parentBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(common.COLOR_LIGHTBLUE)).
				Padding(0, 1)

	// Reply styles (indented)
	replyTimeStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY))

	replyAuthorStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_BLUE)).
				Bold(true)

	replyContentStyle = lipgloss.NewStyle().
				Align(lipgloss.Left)

	// Selected reply styles
	selectedReplyTimeStyle = lipgloss.NewStyle().
				Align(lipgloss.Left).
				Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedReplyAuthorStyle = lipgloss.NewStyle().
					Align(lipgloss.Left).
					Foreground(lipgloss.Color(common.COLOR_WHITE)).
					Bold(true)

	selectedReplyContentStyle = lipgloss.NewStyle().
					Align(lipgloss.Left).
					Foreground(lipgloss.Color(common.COLOR_WHITE))

	selectedBgStyle = lipgloss.NewStyle().
			Background(lipgloss.Color(common.COLOR_LIGHTBLUE))

	emptyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(common.COLOR_DARK_GREY)).
			Italic(true)

	replyIndent = "  " // Indent for replies
)

// ThreadPost represents a post in the thread (either parent or reply)
type ThreadPost struct {
	ID        uuid.UUID
	Author    string
	Content   string
	Time      time.Time
	ObjectURI string
	IsLocal   bool // Whether this is a local post
	IsParent  bool // Whether this is the parent post
	IsDeleted bool // Whether this post was deleted (placeholder)
}

// Model represents the thread view state
type Model struct {
	AccountId    uuid.UUID
	ParentURI    string       // URI of the parent post being viewed
	ParentPost   *ThreadPost  // The parent post
	Replies      []ThreadPost // Replies to the parent
	Selected     int          // Currently selected reply index (-1 = parent selected)
	Width        int
	Height       int
	isActive     bool
	loading      bool
	errorMessage string
}

// InitialModel creates a new thread view model
func InitialModel(accountId uuid.UUID, width, height int) Model {
	return Model{
		AccountId:    accountId,
		ParentURI:    "",
		ParentPost:   nil,
		Replies:      []ThreadPost{},
		Selected:     -1, // Start with parent selected
		Width:        width,
		Height:       height,
		isActive:     false,
		loading:      false,
		errorMessage: "",
	}
}

// SetThread sets the thread to display
func (m *Model) SetThread(parentURI string) {
	m.ParentURI = parentURI
	m.ParentPost = nil
	m.Replies = []ThreadPost{}
	m.Selected = -1
	m.loading = true
	m.errorMessage = ""
}

func (m Model) Init() tea.Cmd {
	return nil
}

// threadLoadedMsg is sent when thread data is loaded
type threadLoadedMsg struct {
	parent  *ThreadPost
	replies []ThreadPost
	err     error
}

// loadThread loads the parent post and its replies
func loadThread(parentURI string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		var parent *ThreadPost
		var replies []ThreadPost

		// Try to find the parent post
		// First check if it's a local note by URI
		err, localNote := database.ReadNoteByURI(parentURI)
		if err == nil && localNote != nil {
			parent = &ThreadPost{
				ID:        localNote.Id,
				Author:    localNote.CreatedBy,
				Content:   localNote.Message,
				Time:      localNote.CreatedAt,
				ObjectURI: localNote.ObjectURI,
				IsLocal:   true,
				IsParent:  true,
			}
		} else {
			// Check if it's a stored activity (federated post)
			err, activity := database.ReadActivityByObjectURI(parentURI)
			if err == nil && activity != nil {
				// Parse activity to get content
				content, author := parseActivityContent(activity)
				parent = &ThreadPost{
					ID:        activity.Id,
					Author:    author,
					Content:   content,
					Time:      activity.CreatedAt,
					ObjectURI: activity.ObjectURI,
					IsLocal:   false,
					IsParent:  true,
				}
			}
		}

		// Load replies using the parentURI (which matches in_reply_to_uri)
		err, localReplies := database.ReadRepliesByURI(parentURI)
		if err == nil && localReplies != nil {
			for _, note := range *localReplies {
				replies = append(replies, ThreadPost{
					ID:        note.Id,
					Author:    note.CreatedBy,
					Content:   note.Message,
					Time:      note.CreatedAt,
					ObjectURI: note.ObjectURI,
					IsLocal:   true,
					IsParent:  false,
				})
			}
		}

		// If parent not found but we have replies, create a deleted placeholder
		if parent == nil {
			if len(replies) > 0 {
				parent = &ThreadPost{
					Author:    "[deleted]",
					Content:   "This post has been deleted",
					IsParent:  true,
					IsDeleted: true,
				}
			} else {
				return threadLoadedMsg{
					parent:  nil,
					replies: nil,
					err:     fmt.Errorf("post not found"),
				}
			}
		}

		return threadLoadedMsg{
			parent:  parent,
			replies: replies,
			err:     nil,
		}
	}
}

// loadThreadByID loads the thread for a local note by its UUID
func loadThreadByID(noteID uuid.UUID, noteURI string, author string, content string, createdAt time.Time) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Create parent from the provided data
		parent := &ThreadPost{
			ID:        noteID,
			Author:    author,
			Content:   content,
			Time:      createdAt,
			ObjectURI: noteURI,
			IsLocal:   true,
			IsParent:  true,
		}

		// Load replies using the noteURI (which matches in_reply_to_uri)
		var replies []ThreadPost
		err, localReplies := database.ReadRepliesByURI(noteURI)
		if err == nil && localReplies != nil {
			for _, note := range *localReplies {
				replies = append(replies, ThreadPost{
					ID:        note.Id,
					Author:    note.CreatedBy,
					Content:   note.Message,
					Time:      note.CreatedAt,
					ObjectURI: note.ObjectURI,
					IsLocal:   true,
					IsParent:  false,
				})
			}
		}

		return threadLoadedMsg{
			parent:  parent,
			replies: replies,
			err:     nil,
		}
	}
}

// parseActivityContent extracts content and author from an activity's raw JSON
func parseActivityContent(activity *domain.Activity) (string, string) {
	// Simple extraction - in real code you'd parse the JSON properly
	content := ""
	author := activity.ActorURI

	// Try to get a better author name from the database
	database := db.GetDB()
	err, remoteAcc := database.ReadRemoteAccountByActorURI(activity.ActorURI)
	if err == nil && remoteAcc != nil {
		author = "@" + remoteAcc.Username + "@" + remoteAcc.Domain
	}

	// Parse the raw JSON to extract content
	// This is a simplified version - the actual parsing happens in timeline
	if activity.RawJSON != "" {
		// Look for "content" field
		if idx := strings.Index(activity.RawJSON, `"content":"`); idx >= 0 {
			start := idx + len(`"content":"`)
			end := strings.Index(activity.RawJSON[start:], `"`)
			if end > 0 {
				content = activity.RawJSON[start : start+end]
				// Unescape basic HTML entities
				content = strings.ReplaceAll(content, "\\n", "\n")
				content = strings.ReplaceAll(content, "\\\"", "\"")
				content = strings.ReplaceAll(content, "&lt;", "<")
				content = strings.ReplaceAll(content, "&gt;", ">")
				content = strings.ReplaceAll(content, "&amp;", "&")
			}
		}
	}

	return content, author
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.DeactivateViewMsg:
		m.isActive = false
		return m, nil

	case common.ActivateViewMsg:
		m.isActive = true
		// Note: Thread loading is already started by ViewThreadMsg, so we don't need to reload here
		// Only reload if we somehow got back to this view with stale data
		return m, nil

	case common.ViewThreadMsg:
		// Load a new thread
		m.SetThread(msg.NoteURI)
		m.isActive = true
		// For local notes, use loadThreadByID which doesn't rely on object_uri in DB
		if msg.IsLocal && msg.NoteID != uuid.Nil {
			return m, loadThreadByID(msg.NoteID, msg.NoteURI, msg.Author, msg.Content, msg.CreatedAt)
		}
		return m, loadThread(msg.NoteURI)

	case threadLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
			log.Printf("Thread load error: %v", msg.err)
		} else {
			m.ParentPost = msg.parent
			m.Replies = msg.replies
			m.Selected = -1 // Select parent
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.Selected > -1 {
				m.Selected--
			}
		case "down", "j":
			if m.Selected < len(m.Replies)-1 {
				m.Selected++
			}
		case "r":
			// Reply to selected post
			if m.Selected == -1 && m.ParentPost != nil && !m.ParentPost.IsDeleted {
				// Reply to parent (only if not deleted)
				preview := m.ParentPost.Content
				if idx := strings.Index(preview, "\n"); idx > 0 {
					preview = preview[:idx]
				}
				return m, func() tea.Msg {
					return common.ReplyToNoteMsg{
						NoteURI: m.ParentPost.ObjectURI,
						Author:  m.ParentPost.Author,
						Preview: preview,
					}
				}
			} else if m.Selected >= 0 && m.Selected < len(m.Replies) {
				// Reply to selected reply
				reply := m.Replies[m.Selected]
				preview := reply.Content
				if idx := strings.Index(preview, "\n"); idx > 0 {
					preview = preview[:idx]
				}
				return m, func() tea.Msg {
					return common.ReplyToNoteMsg{
						NoteURI: reply.ObjectURI,
						Author:  reply.Author,
						Preview: preview,
					}
				}
			}
		case "esc", "q":
			// Go back (handled by supertui)
			return m, func() tea.Msg {
				return common.FederatedTimelineView
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	var s strings.Builder

	// Header
	replyCount := len(m.Replies)
	s.WriteString(common.CaptionStyle.Render(fmt.Sprintf("thread (%d replies)", replyCount)))
	s.WriteString("\n\n")

	if m.loading {
		s.WriteString(emptyStyle.Render("Loading thread..."))
		return s.String()
	}

	if m.errorMessage != "" {
		s.WriteString(emptyStyle.Render("Error: " + m.errorMessage))
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("Press ESC to go back"))
		return s.String()
	}

	if m.ParentPost == nil {
		s.WriteString(emptyStyle.Render("No thread to display"))
		s.WriteString("\n\n")
		s.WriteString(common.HelpStyle.Render("Press ESC to go back"))
		return s.String()
	}

	// Render parent post (always visible, highlighted if selected)
	parentView := m.renderPost(m.ParentPost, m.Selected == -1, false)
	if m.Selected == -1 {
		s.WriteString(parentBorderStyle.Render(parentView))
	} else {
		s.WriteString(parentBorderStyle.
			BorderForeground(lipgloss.Color(common.COLOR_DARK_GREY)).
			Render(parentView))
	}
	s.WriteString("\n\n")

	// Render replies
	if len(m.Replies) == 0 {
		s.WriteString(replyIndent)
		s.WriteString(emptyStyle.Render("No replies yet"))
	} else {
		for i, reply := range m.Replies {
			isSelected := i == m.Selected
			replyView := m.renderPost(&reply, isSelected, true)

			// Add indent for replies
			lines := strings.Split(replyView, "\n")
			for j, line := range lines {
				s.WriteString(replyIndent)
				s.WriteString(line)
				if j < len(lines)-1 {
					s.WriteString("\n")
				}
			}
			s.WriteString("\n\n")
		}
	}

	// Help text
	s.WriteString("\n")
	s.WriteString(common.HelpStyle.Render("r: reply | j/k: navigate | esc: back"))

	return s.String()
}

// renderPost renders a single post
func (m Model) renderPost(post *ThreadPost, isSelected, isReply bool) string {
	// Handle deleted posts with special styling
	if post.IsDeleted {
		var sb strings.Builder
		deletedStyle := emptyStyle
		sb.WriteString(deletedStyle.Render(post.Author))
		sb.WriteString("\n")
		sb.WriteString(deletedStyle.Render(post.Content))
		return sb.String()
	}

	var timeStyle, authorStyle, contentStyle lipgloss.Style

	if isSelected {
		if isReply {
			timeStyle = selectedReplyTimeStyle
			authorStyle = selectedReplyAuthorStyle
			contentStyle = selectedReplyContentStyle
		} else {
			timeStyle = selectedReplyTimeStyle
			authorStyle = selectedReplyAuthorStyle
			contentStyle = selectedReplyContentStyle
		}
	} else {
		if isReply {
			timeStyle = replyTimeStyle
			authorStyle = replyAuthorStyle
			contentStyle = replyContentStyle
		} else {
			timeStyle = parentTimeStyle
			authorStyle = parentAuthorStyle
			contentStyle = parentContentStyle
		}
	}

	// Format content
	content := post.Content
	if post.IsLocal {
		// Process markdown links for local posts
		content = util.MarkdownLinksToTerminal(content)
	}
	content = util.HighlightHashtagsTerminal(content)
	content = util.TruncateVisibleLength(content, 150)

	// Build post view
	var sb strings.Builder

	timeStr := timeStyle.Render(formatTime(post.Time))
	authorStr := authorStyle.Render(post.Author)
	contentStr := contentStyle.Render(content)

	if isSelected {
		// Apply background to selected post
		sb.WriteString(selectedBgStyle.Render(timeStr))
		sb.WriteString("\n")
		sb.WriteString(selectedBgStyle.Render(authorStr))
		sb.WriteString("\n")
		sb.WriteString(selectedBgStyle.Render(contentStr))
	} else {
		sb.WriteString(timeStr)
		sb.WriteString("\n")
		sb.WriteString(authorStr)
		sb.WriteString("\n")
		sb.WriteString(contentStr)
	}

	return sb.String()
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

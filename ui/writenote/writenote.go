package writenote

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deemkeen/stegodon/activitypub"
	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

const MaxLetters = 150

type Model struct {
	Textarea          textarea.Model
	Err               util.ErrMsg
	Error             string // Error message to display
	userId            uuid.UUID
	lettersLeft       int
	width             int
	isEditing         bool      // True when editing an existing note
	editingNoteId     uuid.UUID // ID of note being edited
	originalCreatedAt time.Time // Original creation time (preserved during edit)
	// Reply mode fields
	isReplying     bool   // True when replying to a post
	replyToURI     string // URI of the post being replied to
	replyToAuthor  string // Author of the post being replied to
	replyToPreview string // Preview of the post being replied to
}

func InitialNote(contentWidth int, userId uuid.UUID) Model {
	width := common.DefaultCreateNoteWidth(contentWidth)
	ti := textarea.New()
	ti.Placeholder = "enter your message"
	ti.CharLimit = 1000 // Set to DB limit, we'll validate visible chars separately
	ti.ShowLineNumbers = false
	ti.SetWidth(common.TextInputDefaultWidth)
	ti.Cursor.SetMode(cursor.CursorBlink)
	ti.Focus()

	return Model{
		Textarea:          ti,
		Err:               nil,
		Error:             "",
		userId:            userId,
		lettersLeft:       MaxLetters,
		width:             width,
		isEditing:         false,
		editingNoteId:     uuid.Nil,
		originalCreatedAt: time.Time{},
		isReplying:        false,
		replyToURI:        "",
		replyToAuthor:     "",
		replyToPreview:    "",
	}
}

func createNoteModelCmd(note *domain.SaveNote) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Create note in database and get the created note ID
		// Use CreateNoteWithReply to support replies
		noteId, err := database.CreateNoteWithReply(note.UserId, note.Message, note.InReplyToURI)
		if err != nil {
			log.Println("Note could not be saved!")
			return common.UpdateNoteList
		}

		// Link hashtags to the note
		hashtags := util.ParseHashtags(note.Message)
		if len(hashtags) > 0 {
			hashtagIds := make([]int64, 0, len(hashtags))
			for _, tag := range hashtags {
				hashtagId, err := database.CreateOrUpdateHashtag(tag)
				if err != nil {
					log.Printf("Failed to create/update hashtag %s: %v", tag, err)
					continue
				}
				hashtagIds = append(hashtagIds, hashtagId)
			}
			if len(hashtagIds) > 0 {
				if err := database.LinkNoteHashtags(noteId, hashtagIds); err != nil {
					log.Printf("Failed to link hashtags to note: %v", err)
				}
			}
		}

		// Federate the note via ActivityPub (background task)
		go func() {
			// Get the created note from database with actual ID, timestamps, and reply info
			err, createdNote := database.ReadNoteIdWithReplyInfo(noteId)
			if err != nil {
				log.Printf("Failed to read created note for federation: %v", err)
				return
			}

			// Get the account
			err, account := database.ReadAccById(note.UserId)
			if err != nil {
				log.Printf("Failed to get account for federation: %v", err)
				return
			}

			// Get config
			conf, err := util.ReadConf()
			if err != nil {
				log.Printf("Failed to read config for federation: %v", err)
				return
			}

			// Only federate if ActivityPub is enabled
			if !conf.Conf.WithAp {
				return
			}

			// Send Create activity to all followers with the actual note from database
			if err := activitypub.SendCreate(createdNote, account, conf); err != nil {
				log.Printf("Failed to federate note: %v", err)
			} else {
				log.Printf("Note federated successfully for %s", account.Username)
			}
		}()

		return common.UpdateNoteList
	}
}

func updateNoteModelCmd(noteId uuid.UUID, message string) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Update note in database
		err := database.UpdateNote(noteId, message)
		if err != nil {
			log.Printf("Note could not be updated: %v", err)
			return common.UpdateNoteList
		}

		log.Printf("Note %s updated successfully", noteId)

		// Update hashtags for the note
		// First, we need to unlink old hashtags and link new ones
		// For simplicity, we'll just create/update new hashtags and link them
		// The old links will remain (could be cleaned up with a separate function)
		hashtags := util.ParseHashtags(message)
		if len(hashtags) > 0 {
			hashtagIds := make([]int64, 0, len(hashtags))
			for _, tag := range hashtags {
				hashtagId, err := database.CreateOrUpdateHashtag(tag)
				if err != nil {
					log.Printf("Failed to create/update hashtag %s: %v", tag, err)
					continue
				}
				hashtagIds = append(hashtagIds, hashtagId)
			}
			if len(hashtagIds) > 0 {
				if err := database.LinkNoteHashtags(noteId, hashtagIds); err != nil {
					log.Printf("Failed to link hashtags to note: %v", err)
				}
			}
		}

		// Federate the update via ActivityPub (background task)
		go func() {
			// Get the updated note
			err, note := database.ReadNoteId(noteId)
			if err != nil {
				log.Printf("Failed to get note for federation: %v", err)
				return
			}

			// Get the account
			err, account := database.ReadAccByUsername(note.CreatedBy)
			if err != nil {
				log.Printf("Failed to get account for federation: %v", err)
				return
			}

			// Get config
			conf, err := util.ReadConf()
			if err != nil {
				log.Printf("Failed to read config for federation: %v", err)
				return
			}

			// Only federate if ActivityPub is enabled
			if !conf.Conf.WithAp {
				return
			}

			// Send Update activity to all followers
			if err := activitypub.SendUpdate(note, account, conf); err != nil {
				log.Printf("Failed to federate note update: %v", err)
			} else {
				log.Printf("Note update federated successfully for %s", account.Username)
			}
		}()

		return common.UpdateNoteList
	}
}

func (m Model) Init() tea.Cmd {
	return textarea.Blink
}

func (m *Model) Focus() {
	m.Textarea.Focus()
}

func (m *Model) Blur() {
	m.Textarea.Blur()
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case common.EditNoteMsg:
		// Enter edit mode: populate textarea with existing note
		m.isEditing = true
		m.editingNoteId = msg.NoteId
		m.originalCreatedAt = msg.CreatedAt
		m.Textarea.SetValue(msg.Message)
		m.Textarea.Focus()
		// Clear reply mode if active
		m.isReplying = false
		m.replyToURI = ""
		m.replyToAuthor = ""
		m.replyToPreview = ""
		return m, nil

	case common.ReplyToNoteMsg:
		// Enter reply mode
		m.isReplying = true
		m.replyToURI = msg.NoteURI
		m.replyToAuthor = msg.Author
		m.replyToPreview = msg.Preview
		// Clear edit mode if active
		m.isEditing = false
		m.editingNoteId = uuid.Nil
		m.originalCreatedAt = time.Time{}
		// Clear textarea and focus
		m.Textarea.SetValue("")
		m.Textarea.Focus()
		return m, nil

	case tea.KeyMsg:
		// Clear error when user starts typing
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeyBackspace {
			m.Error = ""
		}

		switch msg.Type {
		case tea.KeyCtrlA:
			if m.Textarea.Focused() {
				m.Textarea.Blur()
			}
		case tea.KeyCtrlS:
			rawValue := m.Textarea.Value()

			// Validate that note is not empty (trim whitespace for validation)
			if len(strings.TrimSpace(rawValue)) == 0 {
				m.Error = "Cannot save an empty note"
				return m, nil
			}

			// Validate that visible characters don't exceed 150
			visibleChars := util.CountVisibleChars(rawValue)
			if visibleChars > MaxLetters {
				m.Error = fmt.Sprintf("Note too long (%d visible characters, max %d)", visibleChars, MaxLetters)
				return m, nil
			}

			// Validate that full message (including markdown) doesn't exceed 1000 chars
			// Check BEFORE normalizing, as normalization might change length
			if err := util.ValidateNoteLength(rawValue); err != nil {
				m.Error = err.Error()
				return m, nil
			}

			// Normalize input after validation
			value := util.NormalizeInput(rawValue)

			if m.isEditing {
				// Update existing note
				noteId := m.editingNoteId
				m.Textarea.SetValue("")
				m.Error = ""
				// Exit edit mode
				m.isEditing = false
				m.editingNoteId = uuid.Nil
				m.originalCreatedAt = time.Time{}
				return m, updateNoteModelCmd(noteId, value)
			} else if m.isReplying {
				// Create reply note with inReplyTo
				note := domain.SaveNote{
					UserId:       m.userId,
					Message:      value,
					InReplyToURI: m.replyToURI,
				}
				m.Textarea.SetValue("")
				m.Error = ""
				// Exit reply mode
				m.isReplying = false
				m.replyToURI = ""
				m.replyToAuthor = ""
				m.replyToPreview = ""
				return m, createNoteModelCmd(&note)
			} else {
				// Create new note
				note := domain.SaveNote{
					UserId:  m.userId,
					Message: value,
				}
				m.Textarea.SetValue("")
				m.Error = ""
				return m, createNoteModelCmd(&note)
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			// Cancel edit mode or reply mode
			if m.isEditing {
				m.isEditing = false
				m.editingNoteId = uuid.Nil
				m.originalCreatedAt = time.Time{}
				m.Textarea.SetValue("")
				return m, nil
			}
			if m.isReplying {
				m.isReplying = false
				m.replyToURI = ""
				m.replyToAuthor = ""
				m.replyToPreview = ""
				m.Textarea.SetValue("")
				return m, nil
			}
		default:
			if !m.Textarea.Focused() {
				cmd = m.Textarea.Focus()
				cmds = append(cmds, cmd)
			}
		}

	// We handle errors just like any other message
	case util.ErrMsg:
		m.Err = msg
		return m, nil
	}

	m.Textarea, cmd = m.Textarea.Update(msg)

	// Check if visible character count exceeds 150
	visibleChars := util.CountVisibleChars(m.Textarea.Value())
	if visibleChars > MaxLetters {
		// Revert the last change by not allowing more visible chars
		// Note: This is a simple check, ideally we'd prevent the input
		// For now, the character counter will show negative and save will fail
	}

	// Auto-convert pasted URLs to markdown format
	// Check if the entire content is a URL (user just pasted a URL)
	currentValue := m.Textarea.Value()
	if util.IsURL(strings.TrimSpace(currentValue)) {
		url := strings.TrimSpace(currentValue)
		// Convert to markdown with "Link" as the text: [Link](url)
		markdown := fmt.Sprintf("[Link](%s)", url)
		m.Textarea.SetValue(markdown)
		// Move cursor to end
		m.Textarea.CursorEnd()
	}

	m.lettersLeft = m.CharCount()
	cmds = append(cmds, cmd)

	// Avoid tea.Batch() when possible to prevent goroutine leaks
	switch len(cmds) {
	case 0:
		return m, nil
	case 1:
		return m, cmds[0]
	default:
		return m, tea.Batch(cmds...)
	}
}

func (m Model) CharCount() int {
	// Use CountVisibleChars to only count visible text, not markdown URLs
	visibleChars := util.CountVisibleChars(m.Textarea.Value())
	return MaxLetters - visibleChars
}

// getMarkdownLinkCount returns the number of valid markdown links in the text
func getMarkdownLinkCount(text string) int {
	re := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	return len(re.FindAllString(text, -1))
}

func (m Model) View() string {
	styledTextarea := lipgloss.NewStyle().PaddingLeft(5).PaddingRight(5).Render(m.Textarea.View())

	// Show markdown link indicator if any are detected
	linkIndicator := ""
	if linkCount := getMarkdownLinkCount(m.Textarea.Value()); linkCount > 0 {
		linkStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			PaddingLeft(5)
		plural := ""
		if linkCount > 1 {
			plural = "s"
		}
		linkIndicator = "\n" + linkStyle.Render(fmt.Sprintf("✓ %d markdown link%s detected", linkCount, plural))
	}

	helpText := "post message: ctrl+s"
	if m.isEditing {
		helpText = "save changes: ctrl+s\ncancel: esc"
	} else if m.isReplying {
		helpText = "post reply: ctrl+s\ncancel: esc"
	}

	// Build the help section with proper formatting
	helpLines := fmt.Sprintf("characters left: %d\n\n%s", m.lettersLeft, helpText)
	charsLeft := common.HelpStyle.Render(lipgloss.NewStyle().PaddingLeft(5).Render(helpLines))

	captionText := "new note"
	if m.isEditing {
		captionText = "edit note"
	} else if m.isReplying {
		captionText = "reply to @" + m.replyToAuthor
	}
	caption := common.CaptionStyle.PaddingLeft(5).Render(captionText)

	// Show reply context if replying
	replyContext := ""
	if m.isReplying && m.replyToPreview != "" {
		replyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Italic(true).
			PaddingLeft(5)
		// Truncate preview if too long
		preview := m.replyToPreview
		if len(preview) > 60 {
			preview = preview[:57] + "..."
		}
		replyContext = replyStyle.Render("\"" + preview + "\"") + "\n\n"
	}

	// Add error message if present
	errorSection := ""
	if m.Error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			PaddingLeft(5)
		errorSection = "\n" + errorStyle.Render(m.Error)
	}

	return fmt.Sprintf("%s\n\n%s%s%s%s\n\n%s", caption, replyContext, styledTextarea, linkIndicator, errorSection, charsLeft)
}

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
	Error             string    // Error message to display
	userId            uuid.UUID
	lettersLeft       int
	width             int
	isEditing         bool      // True when editing an existing note
	editingNoteId     uuid.UUID // ID of note being edited
	originalCreatedAt time.Time // Original creation time (preserved during edit)
}

func InitialNote(contentWidth int, userId uuid.UUID) Model {
	width := common.DefaultCreateNoteWidth(contentWidth)
	ti := textarea.New()
	ti.Placeholder = "enter your message"
	ti.CharLimit = 1000 // Set to DB limit, we'll validate visible chars separately
	ti.ShowLineNumbers = false
	ti.SetWidth(30)
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
	}
}

func createNoteModelCmd(note *domain.SaveNote) tea.Cmd {
	return func() tea.Msg {
		database := db.GetDB()

		// Create note in database and get the created note ID
		noteId, err := database.CreateNote(note.UserId, note.Message)
		if err != nil {
			log.Println("Note could not be saved!")
			return common.UpdateNoteList
		}

		// Federate the note via ActivityPub (background task)
		go func() {
			// Get the created note from database with actual ID and timestamps
			err, createdNote := database.ReadNoteId(noteId)
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
			// Cancel edit mode
			if m.isEditing {
				m.isEditing = false
				m.editingNoteId = uuid.Nil
				m.originalCreatedAt = time.Time{}
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
	return m, tea.Batch(cmds...)
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
	}

	// Build the help section with proper formatting
	helpLines := fmt.Sprintf("characters left: %d\n\n%s", m.lettersLeft, helpText)
	charsLeft := common.HelpStyle.Render(lipgloss.NewStyle().PaddingLeft(5).Render(helpLines))

	captionText := "new note"
	if m.isEditing {
		captionText = "edit note"
	}
	caption := common.CaptionStyle.PaddingLeft(5).Render(captionText)

	// Add error message if present
	errorSection := ""
	if m.Error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true).
			PaddingLeft(5)
		errorSection = "\n" + errorStyle.Render(m.Error)
	}

	return fmt.Sprintf("%s\n\n%s%s%s\n\n%s", caption, styledTextarea, linkIndicator, errorSection, charsLeft)
}

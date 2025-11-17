package writenote

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestEmptyNoteValidation(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Test 1: Try to save empty note
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.Error == "" {
		t.Error("Expected error message when saving empty note")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}

	// Test 2: Type something and verify error clears
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})

	if m.Error != "" {
		t.Errorf("Expected error to clear when typing, but still has: '%s'", m.Error)
	}
}

func TestEmptyNoteWithWhitespaceValidation(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set textarea to only whitespace
	m.Textarea.SetValue("   \n\n  \t  ")

	// Try to save (should fail because NormalizeInput will trim to empty)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.Error == "" {
		t.Error("Expected error message when saving whitespace-only note")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}
}

func TestValidNoteDoesNotShowError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set valid content
	m.Textarea.SetValue("This is a valid note")

	// Try to save (should succeed and clear any previous errors)
	oldValue := m.Textarea.Value()
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.Error != "" {
		t.Errorf("Expected no error for valid note, got: '%s'", m.Error)
	}

	// Textarea should be cleared after save
	if m.Textarea.Value() != "" {
		t.Errorf("Expected textarea to be cleared after save, got: '%s'", m.Textarea.Value())
	}

	// Should return a command (the save command)
	if cmd == nil {
		t.Error("Expected save command to be returned")
	}

	// Verify the original content was valid
	if oldValue == "" {
		t.Error("Test setup error: should have had valid content")
	}
}

func TestErrorClearsOnBackspace(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Try to save empty note to trigger error
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.Error == "" {
		t.Fatal("Setup error: expected error from empty save")
	}

	// Press backspace
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})

	if m.Error != "" {
		t.Errorf("Expected error to clear on backspace, but still has: '%s'", m.Error)
	}
}

func TestErrorInEditMode(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Enter edit mode
	noteId := uuid.New()
	m, _ = m.Update(common.EditNoteMsg{
		NoteId:    noteId,
		Message:   "Original message",
		CreatedAt: time.Now(),
	})

	if !m.isEditing {
		t.Fatal("Setup error: should be in edit mode")
	}

	// Clear the textarea to make it empty
	m.Textarea.SetValue("")

	// Try to save empty note in edit mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.Error == "" {
		t.Error("Expected error when saving empty note in edit mode")
	}
	if m.Error != "Cannot save an empty note" {
		t.Errorf("Expected error 'Cannot save an empty note', got '%s'", m.Error)
	}

	// Should still be in edit mode because save failed
	if !m.isEditing {
		t.Error("Should still be in edit mode after failed save")
	}
}

func TestViewShowsError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Set an error
	m.Error = "Cannot save an empty note"

	// Render the view
	view := m.View()

	// Check that error message appears in view
	if !strings.Contains(view, "Cannot save an empty note") {
		t.Error("Expected error message to appear in view")
	}
}

func TestViewWithoutError(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Render the view without error
	view := m.View()

	// Check that no error styling appears
	if strings.Contains(view, "Cannot save") {
		t.Error("Should not show error message when no error exists")
	}
}

func TestCharCountIsCorrect(t *testing.T) {
	// Create a model
	m := InitialNote(100, uuid.New())

	// Initial count should be MaxLetters
	if m.CharCount() != MaxLetters {
		t.Errorf("Expected initial char count to be %d, got %d", MaxLetters, m.CharCount())
	}

	// Add some text
	testText := "Hello"
	m.Textarea.SetValue(testText)
	m.lettersLeft = m.CharCount()

	expectedLeft := MaxLetters - len(testText)
	if m.lettersLeft != expectedLeft {
		t.Errorf("Expected %d characters left after typing '%s', got %d",
			expectedLeft, testText, m.lettersLeft)
	}
}

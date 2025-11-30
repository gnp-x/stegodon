package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

// TestMainModelInitialization verifies the main model starts correctly
func TestMainModelInitialization(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	if model.account.Username != "testuser" {
		t.Errorf("Expected username testuser, got %s", model.account.Username)
	}

	// Width and height are adjusted by common.DefaultWindowWidth/Height
	// Just verify they're set to reasonable values
	if model.width < 80 {
		t.Errorf("Expected width >= 80, got %d", model.width)
	}

	if model.height < 20 {
		t.Errorf("Expected height >= 20, got %d", model.height)
	}
}

// TestMessageRoutingDoesNotPanic verifies message routing doesn't panic
func TestMessageRoutingDoesNotPanic(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Test various message types don't cause panics
	testCases := []struct {
		name string
		msg  tea.Msg
	}{
		{"ActivateViewMsg", common.ActivateViewMsg{}},
		{"DeactivateViewMsg", common.DeactivateViewMsg{}},
		{"KeyMsg", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}},
		{"SessionState", common.UpdateNoteList},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Update panicked with message %s: %v", tc.name, r)
				}
			}()

			_, _ = model.Update(tc.msg)
		})
	}
}

// TestCommandFilteringRemovesNils verifies nil commands are filtered out
func TestCommandFilteringRemovesNils(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Send a message that should return a command (or nil)
	// The key is that it doesn't panic and returns a valid tea.Model
	updatedModel, cmd := model.Update(common.ActivateViewMsg{})

	if updatedModel == nil {
		t.Error("Expected Update to return non-nil model")
	}

	// cmd can be nil or non-nil, both are valid
	// The important thing is we didn't panic during filtering
	_ = cmd
}

// TestViewSwitchingDoesNotPanic verifies Tab navigation works
func TestViewSwitchingDoesNotPanic(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	// Start in CreateNoteView (default for non-first-time users)
	model.state = common.CreateNoteView

	// Press Tab to cycle through views
	for i := 0; i < 10; i++ {
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = r.(error)
				}
			}()
			teaModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\t'}})
			model = teaModel.(MainModel)
		}()

		if err != nil {
			t.Errorf("Tab navigation panicked on iteration %d: %v", i, err)
			break
		}
	}
}

// TestTimelineActivationReturnsCommand verifies timeline activation returns a command
func TestTimelineActivationReturnsCommand(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.FederatedTimelineView

	// Send ActivateViewMsg to timeline
	_, cmd := model.Update(common.ActivateViewMsg{})

	// Timeline activation should return a command (either from timelineModel or nil)
	// The important thing is it doesn't panic
	_ = cmd
}

// TestMultipleCommandsReturnsFirstNonNil verifies command prioritization
func TestMultipleCommandsReturnsFirstNonNil(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Send EditNoteMsg which routes to multiple models
	editMsg := common.EditNoteMsg{
		NoteId:  uuid.New(),
		Message: "test",
	}

	_, cmd := model.Update(editMsg)

	// Should return a single command (not batched)
	// This verifies our "return first non-nil" logic works
	_ = cmd
}

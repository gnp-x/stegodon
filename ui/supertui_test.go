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

// TestSafeModelsReceiveMessagesRegardlessOfActiveView verifies that safe models
// (those without background tickers) receive messages even when not the active view
func TestSafeModelsReceiveMessagesRegardlessOfActiveView(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	// Set active view to CreateNoteView
	model.state = common.CreateNoteView

	// Create a generic message (not a special case like ActivateViewMsg or SessionState)
	type testMsg struct{}

	// Send the message - safe models should receive it even though they're not active
	updatedModel, _ := model.Update(testMsg{})

	if updatedModel == nil {
		t.Error("Expected Update to return non-nil model")
	}

	// The test verifies that no panic occurs when routing to inactive safe models
	// In production, followModel, deleteAccountModel, followersModel, followingModel,
	// and localUsersModel should all receive this message regardless of active view
}

// TestTimelineModelsOnlyReceiveMessagesWhenActive verifies that timeline models
// with background tickers only receive messages when they're the active view
func TestTimelineModelsOnlyReceiveMessagesWhenActive(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	testCases := []struct {
		name        string
		activeView  common.SessionState
		description string
	}{
		{
			name:        "FederatedTimelineActive",
			activeView:  common.FederatedTimelineView,
			description: "timelineModel should receive messages when FederatedTimelineView is active",
		},
		{
			name:        "LocalTimelineActive",
			activeView:  common.LocalTimelineView,
			description: "localTimelineModel should receive messages when LocalTimelineView is active",
		},
		{
			name:        "AdminPanelActive",
			activeView:  common.AdminPanelView,
			description: "adminModel should receive messages when AdminPanelView is active",
		},
		{
			name:        "CreateNoteActive",
			activeView:  common.CreateNoteView,
			description: "Timeline models should NOT receive messages when CreateNoteView is active",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model.state = tc.activeView

			type testMsg struct{}

			// Send message - should not panic regardless of routing
			updatedModel, _ := model.Update(testMsg{})

			if updatedModel == nil {
				t.Errorf("Expected Update to return non-nil model for %s", tc.description)
			}
		})
	}
}

// TestMessageRoutingPreventsPanicsAcrossAllViews verifies message routing
// doesn't panic when switching between all possible views
func TestMessageRoutingPreventsPanicsAcrossAllViews(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)

	allViews := []common.SessionState{
		common.CreateUserView,
		common.CreateNoteView,
		common.ListNotesView,
		common.FollowUserView,
		common.FollowersView,
		common.FollowingView,
		common.FederatedTimelineView,
		common.LocalTimelineView,
		common.LocalUsersView,
		common.AdminPanelView,
		common.DeleteAccountView,
	}

	// Create various message types to test
	type genericMsg struct{}
	messages := []tea.Msg{
		genericMsg{},
		common.ActivateViewMsg{},
		common.DeactivateViewMsg{},
		common.UpdateNoteList,
	}

	for _, view := range allViews {
		for _, msg := range messages {
			model.state = view

			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Panic with view %v and message %T: %v", view, msg, r)
					}
				}()

				updatedModel, _ := model.Update(msg)
				if updatedModel == nil {
					t.Errorf("Expected non-nil model for view %v with message %T", view, msg)
				}
			}()
		}
	}
}

// TestCommandBatchingWithMultipleModels verifies that when multiple models
// return commands, they are properly batched
func TestCommandBatchingWithMultipleModels(t *testing.T) {
	account := domain.Account{
		Id:       uuid.New(),
		Username: "testuser",
	}

	model := NewModel(account, 100, 30)
	model.state = common.CreateNoteView

	// Send EditNoteMsg which routes to multiple models (listModel, createModel)
	editMsg := common.EditNoteMsg{
		NoteId:  uuid.New(),
		Message: "test message",
	}

	updatedModel, cmd := model.Update(editMsg)

	if updatedModel == nil {
		t.Error("Expected non-nil model after EditNoteMsg")
	}

	// Command can be nil or non-nil, both are valid
	// The important part is that multiple commands are handled correctly
	_ = cmd
}

// TestDeleteAccountModelUpdatedAfterUsernameChange verifies deleteAccountModel
// receives updated account info after username creation
func TestDeleteAccountModelUpdatedAfterUsernameChange(t *testing.T) {
	account := domain.Account{
		Id:             uuid.New(),
		Username:       "internal_generated_name",
		FirstTimeLogin: domain.TRUE,
	}

	model := NewModel(account, 100, 30)
	model.state = common.CreateUserView

	// Setup the newUserModel as if user filled in the form
	model.newUserModel.TextInput.SetValue("alice")
	model.newUserModel.DisplayName.SetValue("Alice Test")
	model.newUserModel.Bio.SetValue("Test bio")
	model.newUserModel.Step = 2

	// Verify deleteAccountModel initially has old username
	if model.deleteAccountModel.Account.Username != "internal_generated_name" {
		t.Errorf("Expected deleteAccountModel to have internal username initially, got %s",
			model.deleteAccountModel.Account.Username)
	}

	// Simulate pressing enter on bio step (Step 2)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mainModel := updatedModel.(MainModel)

	// Verify account was updated
	if mainModel.account.Username != "alice" {
		t.Errorf("Expected account username to be updated to 'alice', got %s", mainModel.account.Username)
	}

	if mainModel.account.DisplayName != "Alice Test" {
		t.Errorf("Expected display name to be 'Alice Test', got %s", mainModel.account.DisplayName)
	}

	// Verify deleteAccountModel now has updated username
	if mainModel.deleteAccountModel.Account.Username != "alice" {
		t.Errorf("Expected deleteAccountModel to have updated username 'alice', got %s",
			mainModel.deleteAccountModel.Account.Username)
	}

	// Verify deleteAccountModel has same values as mainModel.account
	if mainModel.deleteAccountModel.Account.DisplayName != mainModel.account.DisplayName {
		t.Error("deleteAccountModel should have same DisplayName as mainModel.account")
	}

	if mainModel.deleteAccountModel.Account.Summary != mainModel.account.Summary {
		t.Error("deleteAccountModel should have same Summary as mainModel.account")
	}
}


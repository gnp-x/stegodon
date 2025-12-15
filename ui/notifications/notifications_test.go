package notifications

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 100
	height := 40

	model := InitialModel(accountId, width, height)

	if model.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, model.AccountId)
	}
	if model.Width != width {
		t.Errorf("Expected Width %d, got %d", width, model.Width)
	}
	if model.Height != height {
		t.Errorf("Expected Height %d, got %d", height, model.Height)
	}
	if model.isActive != false {
		t.Errorf("Expected isActive false initially, got %v", model.isActive)
	}
	if len(model.Notifications) != 0 {
		t.Errorf("Expected empty notifications list initially")
	}
}

func TestUpdate_ActivateViewMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.isActive = false

	newModel, cmd := model.Update(common.ActivateViewMsg{})

	if !newModel.isActive {
		t.Errorf("Expected isActive true after ActivateViewMsg")
	}
	if cmd == nil {
		t.Errorf("Expected cmd to load notifications")
	}
}

func TestUpdate_DeactivateViewMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.isActive = true

	newModel, cmd := model.Update(common.DeactivateViewMsg{})

	// Note: After our changes, deactivation doesn't actually stop the ticker
	// It just marks the view as not actively viewing
	if newModel.isActive != false {
		t.Errorf("Expected isActive false after DeactivateViewMsg")
	}
	// Cmd should be nil (we don't stop the ticker anymore)
	if cmd != nil {
		t.Errorf("Expected no cmd after DeactivateViewMsg")
	}
}

func TestUpdate_NotificationsLoadedMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	notifications := []domain.Notification{
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationLike,
			ActorUsername:    "user1",
			Read:             false,
			CreatedAt:        time.Now(),
		},
		{
			Id:               uuid.New(),
			AccountId:        model.AccountId,
			NotificationType: domain.NotificationFollow,
			ActorUsername:    "user2",
			Read:             true,
			CreatedAt:        time.Now(),
		},
	}

	msg := notificationsLoadedMsg{
		notifications: notifications,
		unreadCount:   1,
	}

	newModel, cmd := model.Update(msg)

	if len(newModel.Notifications) != 2 {
		t.Errorf("Expected 2 notifications, got %d", len(newModel.Notifications))
	}
	if newModel.UnreadCount != 1 {
		t.Errorf("Expected unread count 1, got %d", newModel.UnreadCount)
	}
	// Should return ticker command to keep refreshing
	if cmd == nil {
		t.Errorf("Expected ticker cmd after loading notifications")
	}
}

func TestUpdate_RefreshTickMsg(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Ticker should always trigger refresh (even if not active)
	_, cmd := model.Update(refreshTickMsg{})

	if cmd == nil {
		t.Errorf("Expected load notifications cmd after refresh tick")
	}

	// Same behavior whether active or not (this is the key change)
	model.isActive = false
	_, cmd = model.Update(refreshTickMsg{})

	if cmd == nil {
		t.Errorf("Expected load notifications cmd even when inactive (for badge updates)")
	}
}

func TestUpdate_KeyboardNavigation(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add some notifications
	model.Notifications = []domain.Notification{
		{Id: uuid.New(), ActorUsername: "user1"},
		{Id: uuid.New(), ActorUsername: "user2"},
		{Id: uuid.New(), ActorUsername: "user3"},
	}
	model.Selected = 0

	// Test down navigation
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected 1 after 'j', got %d", newModel.Selected)
	}

	// Test up navigation
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected 0 after 'k', got %d", newModel.Selected)
	}

	// Test down with arrow key
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected 1 after down arrow, got %d", newModel.Selected)
	}

	// Test up with arrow key
	newModel, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected 0 after up arrow, got %d", newModel.Selected)
	}
}

func TestUpdate_SelectionBounds(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)

	// Add some notifications
	model.Notifications = []domain.Notification{
		{Id: uuid.New(), ActorUsername: "user1"},
		{Id: uuid.New(), ActorUsername: "user2"},
	}
	model.Selected = 0

	// Try to go up from 0 (should stay at 0)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if newModel.Selected != 0 {
		t.Errorf("Expected selected to stay at 0 when at top")
	}

	// Go to last item
	model.Selected = 1
	// Try to go down from last (should stay at last)
	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if newModel.Selected != 1 {
		t.Errorf("Expected selected to stay at 1 when at bottom")
	}
}

func TestView_EmptyNotifications(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.UnreadCount = 0

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty")
	}
	// Should show empty message
	if len(model.Notifications) == 0 && view == "" {
		t.Errorf("Should render something even with no notifications")
	}
}

func TestView_WithNotifications(t *testing.T) {
	model := InitialModel(uuid.New(), 100, 40)
	model.Notifications = []domain.Notification{
		{
			Id:               uuid.New(),
			NotificationType: domain.NotificationLike,
			ActorUsername:    "alice",
			ActorDomain:      "",
			Read:             false,
			CreatedAt:        time.Now(),
		},
	}
	model.UnreadCount = 1

	view := model.View()

	if view == "" {
		t.Errorf("View should not be empty with notifications")
	}
}

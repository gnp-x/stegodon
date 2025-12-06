package localtimeline

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

// TestLocalTimelineInitialization verifies the initial model state
func TestLocalTimelineInitialization(t *testing.T) {
	accountId := uuid.New()
	width, height := 100, 30

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

	if model.isActive {
		t.Error("Expected isActive to be false initially")
	}

	if len(model.Posts) != 0 {
		t.Error("Expected Posts to be empty initially")
	}

	if model.Offset != 0 {
		t.Error("Expected Offset to be 0 initially")
	}
}

// TestInit verifies Init() returns nil (no commands, preventing goroutine leak)
func TestInit(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	cmd := model.Init()

	// Init should return nil to prevent tea.Batch() goroutine leak
	// The ActivateViewMsg handler starts the refresh cycle instead
	if cmd != nil {
		t.Error("Expected Init() to return nil to prevent goroutine leak")
	}
}

// TestActivateViewMsg verifies activation sets isActive and loads data
func TestActivateViewMsg(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Initially inactive
	if model.isActive {
		t.Fatal("Model should start inactive")
	}

	// Send ActivateViewMsg
	updatedModel, cmd := model.Update(common.ActivateViewMsg{})

	// Should now be active
	if !updatedModel.isActive {
		t.Error("Expected isActive to be true after ActivateViewMsg")
	}

	// Should return a command (loadLocalPosts)
	if cmd == nil {
		t.Error("Expected ActivateViewMsg to return load command")
	}
}

// TestDeactivateViewMsg verifies deactivation sets isActive to false
func TestDeactivateViewMsg(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Activate first
	model.isActive = true

	// Send DeactivateViewMsg
	updatedModel, cmd := model.Update(common.DeactivateViewMsg{})

	// Should now be inactive
	if updatedModel.isActive {
		t.Error("Expected isActive to be false after DeactivateViewMsg")
	}

	// Should return nil command (no more ticks)
	if cmd != nil {
		t.Error("Expected DeactivateViewMsg to return nil command")
	}
}

// TestRefreshTickMsg_WhenActive verifies tick loads data when active
func TestRefreshTickMsg_WhenActive(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = true

	// Send refreshTickMsg
	updatedModel, cmd := model.Update(refreshTickMsg{})

	// Model should still be active
	if !updatedModel.isActive {
		t.Error("Expected model to remain active")
	}

	// Should return load command (not tick command)
	// This is the key fix: tick doesn't spawn another tick, only loads data
	if cmd == nil {
		t.Error("Expected refreshTickMsg to return load command when active")
	}
}

// TestRefreshTickMsg_WhenInactive verifies tick stops when inactive
func TestRefreshTickMsg_WhenInactive(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = false

	// Send refreshTickMsg
	updatedModel, cmd := model.Update(refreshTickMsg{})

	// Model should remain inactive
	if updatedModel.isActive {
		t.Error("Expected model to remain inactive")
	}

	// Should return nil command (stop tick chain)
	if cmd != nil {
		t.Error("Expected refreshTickMsg to return nil when inactive (stop ticker)")
	}
}

// TestPostsLoadedMsg_SchedulesNextTick verifies data load schedules next tick
func TestPostsLoadedMsg_SchedulesNextTick(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = true

	// Create mock posts
	mockPosts := []domain.Note{
		{
			Id:        uuid.New(),
			Message:   "Test post 1",
			CreatedAt: time.Now(),
			CreatedBy: "alice",
		},
		{
			Id:        uuid.New(),
			Message:   "Test post 2",
			CreatedAt: time.Now(),
			CreatedBy: "bob",
		},
	}

	// Send postsLoadedMsg
	msg := postsLoadedMsg{posts: mockPosts}
	updatedModel, cmd := model.Update(msg)

	// Posts should be set
	if len(updatedModel.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(updatedModel.Posts))
	}

	// Should return tickRefresh command (schedule next refresh)
	// This is the key fix: tick is scheduled AFTER data loads, not during tick
	if cmd == nil {
		t.Error("Expected postsLoadedMsg to return tick command when active")
	}
}

// TestPostsLoadedMsg_NoTickWhenInactive verifies no tick when inactive
func TestPostsLoadedMsg_NoTickWhenInactive(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = false

	// Create mock posts
	mockPosts := []domain.Note{
		{
			Id:        uuid.New(),
			Message:   "Test post",
			CreatedAt: time.Now(),
			CreatedBy: "alice",
		},
	}

	// Send postsLoadedMsg
	msg := postsLoadedMsg{posts: mockPosts}
	updatedModel, cmd := model.Update(msg)

	// Posts should be set
	if len(updatedModel.Posts) != 1 {
		t.Errorf("Expected 1 post, got %d", len(updatedModel.Posts))
	}

	// Should NOT return tick command when inactive
	if cmd != nil {
		t.Error("Expected postsLoadedMsg to return nil when inactive (no tick scheduling)")
	}
}

// TestNavigationUpDown verifies up/down navigation
func TestNavigationUpDown(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Add mock posts
	model.Posts = []domain.Note{
		{Id: uuid.New(), Message: "Post 1", CreatedAt: time.Now(), CreatedBy: "user1"},
		{Id: uuid.New(), Message: "Post 2", CreatedAt: time.Now(), CreatedBy: "user2"},
		{Id: uuid.New(), Message: "Post 3", CreatedAt: time.Now(), CreatedBy: "user3"},
	}
	model.Selected = 1
	model.Offset = 1

	// Press down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updatedModel.Selected != 2 {
		t.Errorf("Expected Selected=2 after down, got %d", updatedModel.Selected)
	}
	if updatedModel.Offset != 2 {
		t.Errorf("Expected Offset=2 after down, got %d", updatedModel.Offset)
	}

	// Press up
	updatedModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updatedModel.Selected != 1 {
		t.Errorf("Expected Selected=1 after up, got %d", updatedModel.Selected)
	}

	// Press up again
	updatedModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updatedModel.Selected != 0 {
		t.Errorf("Expected Selected=0 after up, got %d", updatedModel.Selected)
	}

	// Try to go below 0 (should stay at 0)
	updatedModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyUp})
	if updatedModel.Selected != 0 {
		t.Errorf("Expected Selected to stay at 0, got %d", updatedModel.Selected)
	}
}

// TestNavigationBounds verifies navigation stays within post bounds
func TestNavigationBounds(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Add 3 posts
	model.Posts = []domain.Note{
		{Id: uuid.New(), Message: "Post 1", CreatedAt: time.Now()},
		{Id: uuid.New(), Message: "Post 2", CreatedAt: time.Now()},
		{Id: uuid.New(), Message: "Post 3", CreatedAt: time.Now()},
	}
	model.Selected = 2 // Last post
	model.Offset = 2

	// Try to go down (should stay at last post)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updatedModel.Selected != 2 {
		t.Errorf("Expected Selected to stay at 2 (last post), got %d", updatedModel.Selected)
	}
}

// TestEmptyTimeline verifies handling of empty post list
func TestEmptyTimeline(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// No posts
	model.Posts = []domain.Note{}

	// Try to navigate down (should not panic)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updatedModel.Offset != 0 {
		t.Errorf("Expected Offset=0 for empty timeline, got %d", updatedModel.Offset)
	}

	// Verify View doesn't panic with empty posts
	view := model.View()
	if view == "" {
		t.Error("Expected View to return non-empty string even with no posts")
	}
}

// TestFormatTime verifies relative time formatting
func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		time time.Time
		want string
	}{
		{
			name: "just now",
			time: now.Add(-30 * time.Second),
			want: "just now",
		},
		{
			name: "5 minutes ago",
			time: now.Add(-5 * time.Minute),
			want: "5m ago",
		},
		{
			name: "2 hours ago",
			time: now.Add(-2 * time.Hour),
			want: "2h ago",
		},
		{
			name: "3 days ago",
			time: now.Add(-72 * time.Hour),
			want: "3d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTime(tt.time)
			if got != tt.want {
				t.Errorf("formatTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestTruncate verifies text truncation
func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short text",
			input:  "Hello",
			maxLen: 10,
			want:   "Hello",
		},
		{
			name:   "exact length",
			input:  "HelloWorld",
			maxLen: 10,
			want:   "HelloWorld",
		},
		{
			name:   "needs truncation",
			input:  "This is a long text that needs truncation",
			maxLen: 20,
			want:   "This is a long te...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestMin verifies min function
func TestMin(t *testing.T) {
	tests := []struct {
		a    int
		b    int
		want int
	}{
		{a: 5, b: 10, want: 5},
		{a: 10, b: 5, want: 5},
		{a: 7, b: 7, want: 7},
		{a: -5, b: 3, want: -5},
	}

	for _, tt := range tests {
		got := min(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("min(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// TestPostsLoadedMsg_PreservesOffset verifies offset isn't reset on auto-refresh
func TestPostsLoadedMsg_PreservesOffset(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = true

	// Set initial selection (Selected now drives Offset)
	model.Selected = 1
	model.Offset = 1

	// Load new posts (simulating auto-refresh)
	mockPosts := []domain.Note{
		{Id: uuid.New(), Message: "Post 1", CreatedAt: time.Now()},
		{Id: uuid.New(), Message: "Post 2", CreatedAt: time.Now()},
		{Id: uuid.New(), Message: "Post 3", CreatedAt: time.Now()},
	}

	msg := postsLoadedMsg{posts: mockPosts}
	updatedModel, _ := model.Update(msg)

	// Selected and Offset should stay at 1 (within bounds of 3 posts)
	if updatedModel.Selected != 1 {
		t.Errorf("Expected Selected to be preserved as 1, got %d", updatedModel.Selected)
	}
	if updatedModel.Offset != 1 {
		t.Errorf("Expected Offset to sync with Selected as 1, got %d", updatedModel.Offset)
	}
}

func TestPostsLoadedMsg_BoundsSelection(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = true

	// Set selection beyond what will be available
	model.Selected = 5
	model.Offset = 5

	// Load fewer posts than selected index
	mockPosts := []domain.Note{
		{Id: uuid.New(), Message: "Post 1", CreatedAt: time.Now()},
		{Id: uuid.New(), Message: "Post 2", CreatedAt: time.Now()},
	}

	msg := postsLoadedMsg{posts: mockPosts}
	updatedModel, _ := model.Update(msg)

	// Selected should be bounded to last post (index 1)
	if updatedModel.Selected != 1 {
		t.Errorf("Expected Selected to be bounded to 1, got %d", updatedModel.Selected)
	}
	if updatedModel.Offset != 1 {
		t.Errorf("Expected Offset to sync with bounded Selected, got %d", updatedModel.Offset)
	}
}

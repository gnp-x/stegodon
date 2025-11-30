package timeline

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

// TestTimelineInitialization verifies the initial model state
func TestTimelineInitialization(t *testing.T) {
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

	// Should return a command (loadFederatedPosts)
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
	mockPosts := []FederatedPost{
		{
			Actor:     "@alice@example.com",
			Content:   "Test post 1",
			Time:      time.Now(),
			ObjectURI: "https://example.com/posts/1",
		},
		{
			Actor:     "@bob@example.com",
			Content:   "Test post 2",
			Time:      time.Now(),
			ObjectURI: "https://example.com/posts/2",
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
	mockPosts := []FederatedPost{
		{
			Actor:     "@alice@example.com",
			Content:   "Test post",
			Time:      time.Now(),
			ObjectURI: "https://example.com/posts/1",
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
	model.Posts = []FederatedPost{
		{Actor: "@user1", Content: "Post 1", Time: time.Now()},
		{Actor: "@user2", Content: "Post 2", Time: time.Now()},
		{Actor: "@user3", Content: "Post 3", Time: time.Now()},
	}
	model.Selected = 1
	model.Offset = 1

	// Press down
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updatedModel.Selected != 2 {
		t.Errorf("Expected Selected=2 after down, got %d", updatedModel.Selected)
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

// TestStripHTMLTags verifies HTML tag removal
func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text",
			input: "Hello world",
			want:  "Hello world",
		},
		{
			name:  "paragraph tags",
			input: "<p>Hello world</p>",
			want:  "Hello world",
		},
		{
			name:  "link tag",
			input: `<a href="https://example.com">Link</a>`,
			want:  "Link",
		},
		{
			name:  "HTML entities",
			input: "Test &lt;tag&gt; &amp; &quot;quotes&quot;",
			want:  "Test <tag> & \"quotes\"",
		},
		{
			name:  "multiple tags",
			input: "<p><strong>Bold</strong> and <em>italic</em></p>",
			want:  "Bold and italic",
		},
		{
			name:  "nested tags",
			input: "<div><p><span>Nested</span></p></div>",
			want:  "Nested",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripHTMLTags(tt.input)
			if got != tt.want {
				t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
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

// TestPostsLoadedMsg_SelectionBounds verifies selection stays within bounds
func TestPostsLoadedMsg_SelectionBounds(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)
	model.isActive = true

	// Set Selected beyond bounds
	model.Selected = 10

	// Load only 3 posts
	mockPosts := []FederatedPost{
		{Actor: "@user1", Content: "Post 1", Time: time.Now()},
		{Actor: "@user2", Content: "Post 2", Time: time.Now()},
		{Actor: "@user3", Content: "Post 3", Time: time.Now()},
	}

	msg := postsLoadedMsg{posts: mockPosts}
	updatedModel, _ := model.Update(msg)

	// Selected should be clamped to valid range (0-2)
	if updatedModel.Selected >= len(mockPosts) {
		t.Errorf("Expected Selected < %d, got %d", len(mockPosts), updatedModel.Selected)
	}

	// Should be set to last valid index (2)
	if updatedModel.Selected != 2 {
		t.Errorf("Expected Selected=2 (last post), got %d", updatedModel.Selected)
	}
}

// TestURLToggle verifies 'o' key toggles URL display
func TestURLToggle(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Add mock post with URL
	model.Posts = []FederatedPost{
		{
			Actor:     "@alice@example.com",
			Content:   "Check out this cool link",
			Time:      time.Now(),
			ObjectURI: "https://example.com/post/123",
		},
	}
	model.Selected = 0

	// Initially showing content
	if model.showingURL {
		t.Error("Expected showingURL to be false initially")
	}

	// Press 'o' - should toggle to URL
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !updatedModel.showingURL {
		t.Error("Expected showingURL to be true after pressing 'o'")
	}

	// Press 'o' again - should toggle back to content
	updatedModel, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updatedModel.showingURL {
		t.Error("Expected showingURL to be false after pressing 'o' again")
	}
}

// TestURLResetOnNavigation verifies navigation resets URL display
func TestURLResetOnNavigation(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Add multiple posts
	model.Posts = []FederatedPost{
		{Actor: "@alice", Content: "Post 1", Time: time.Now(), ObjectURI: "https://example.com/1"},
		{Actor: "@bob", Content: "Post 2", Time: time.Now(), ObjectURI: "https://example.com/2"},
	}
	model.Selected = 0

	// Toggle to URL mode
	model.showingURL = true

	// Navigate down - should reset to content mode
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	if updatedModel.showingURL {
		t.Error("Expected showingURL to reset to false when navigating")
	}
	if updatedModel.Selected != 1 {
		t.Errorf("Expected Selected=1, got %d", updatedModel.Selected)
	}
}

// TestURLToggleWithNoURL verifies toggle does nothing when ObjectURI is empty
func TestURLToggleWithNoURL(t *testing.T) {
	accountId := uuid.New()
	model := InitialModel(accountId, 100, 30)

	// Add post WITHOUT URL
	model.Posts = []FederatedPost{
		{
			Actor:     "@alice",
			Content:   "Post without link",
			Time:      time.Now(),
			ObjectURI: "", // Empty URL
		},
	}
	model.Selected = 0

	// Press 'o' - should NOT toggle (no URL to show)
	updatedModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if updatedModel.showingURL {
		t.Error("Expected showingURL to remain false when ObjectURI is empty")
	}
}

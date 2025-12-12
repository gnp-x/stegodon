package hometimeline

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/ui/common"
	"github.com/google/uuid"
)

func TestInitialModel(t *testing.T) {
	accountId := uuid.New()
	width := 120
	height := 40

	m := InitialModel(accountId, width, height, "")

	if m.AccountId != accountId {
		t.Errorf("Expected AccountId %v, got %v", accountId, m.AccountId)
	}
	if m.Width != width {
		t.Errorf("Expected Width %d, got %d", width, m.Width)
	}
	if m.Height != height {
		t.Errorf("Expected Height %d, got %d", height, m.Height)
	}
	if len(m.Posts) != 0 {
		t.Errorf("Expected empty Posts, got %d", len(m.Posts))
	}
	if m.Offset != 0 {
		t.Errorf("Expected Offset 0, got %d", m.Offset)
	}
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0, got %d", m.Selected)
	}
	if m.isActive {
		t.Error("Expected isActive to be false initially")
	}
	if m.showingURL {
		t.Error("Expected showingURL to be false initially")
	}
}

func TestUpdate_ActivateDeactivate(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	// Activate
	m, cmd := m.Update(common.ActivateViewMsg{})
	if !m.isActive {
		t.Error("Expected isActive true after ActivateViewMsg")
	}
	if cmd == nil {
		t.Error("Expected command to load posts on activation")
	}

	// Deactivate
	m, cmd = m.Update(common.DeactivateViewMsg{})
	if m.isActive {
		t.Error("Expected isActive false after DeactivateViewMsg")
	}
	if cmd != nil {
		t.Error("Expected no command on deactivation")
	}
}

func TestUpdate_PostsLoaded(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = true

	posts := []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "First post",
			Time:       time.Now(),
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 0,
		},
		{
			NoteID:     uuid.New(),
			Author:     "@remote@example.com",
			Content:    "Remote post",
			Time:       time.Now(),
			ObjectURI:  "https://remote.example.com/notes/2",
			IsLocal:    false,
			ReplyCount: 3,
		},
	}

	m, cmd := m.Update(postsLoadedMsg{posts: posts})

	if len(m.Posts) != 2 {
		t.Errorf("Expected 2 posts, got %d", len(m.Posts))
	}
	if m.Posts[0].Author != "testuser" {
		t.Errorf("Expected first author 'testuser', got '%s'", m.Posts[0].Author)
	}
	if cmd == nil {
		t.Error("Expected tickRefresh command when active")
	}
}

func TestUpdate_PostsLoaded_InactiveNoTick(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = false // Inactive

	posts := []domain.HomePost{
		{NoteID: uuid.New(), Author: "test", Content: "Test"},
	}

	m, cmd := m.Update(postsLoadedMsg{posts: posts})

	if cmd != nil {
		t.Error("Expected no tick command when inactive")
	}
}

func TestUpdate_RefreshTick_Active(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = true

	_, cmd := m.Update(refreshTickMsg{})

	if cmd == nil {
		t.Error("Expected loadHomePosts command when active")
	}
}

func TestUpdate_RefreshTick_Inactive(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.isActive = false

	_, cmd := m.Update(refreshTickMsg{})

	if cmd != nil {
		t.Error("Expected no command when inactive - ticker should stop")
	}
}

func TestUpdate_UpdateNoteList(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")

	_, cmd := m.Update(common.UpdateNoteList)

	if cmd == nil {
		t.Error("Expected loadHomePosts command on UpdateNoteList")
	}
}

func TestUpdate_Navigation(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1"},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2"},
		{NoteID: uuid.New(), Author: "user3", Content: "Post 3"},
	}
	m.Selected = 0

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'j', got %d", m.Selected)
	}
	if m.showingURL {
		t.Error("Expected showingURL reset after navigation")
	}

	// Move down again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 after down, got %d", m.Selected)
	}

	// Try to move past last (should stay)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if m.Selected != 2 {
		t.Errorf("Expected Selected 2 (stay at last), got %d", m.Selected)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.Selected != 1 {
		t.Errorf("Expected Selected 1 after 'k', got %d", m.Selected)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 after up, got %d", m.Selected)
	}

	// Try to move before first (should stay)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	if m.Selected != 0 {
		t.Errorf("Expected Selected 0 (stay at first), got %d", m.Selected)
	}
}

func TestUpdate_ToggleURL(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post",
			ObjectURI: "https://example.com/notes/1",
		},
	}
	m.Selected = 0

	// Toggle URL on
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !m.showingURL {
		t.Error("Expected showingURL true after 'o'")
	}

	// Toggle URL off
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if m.showingURL {
		t.Error("Expected showingURL false after second 'o'")
	}
}

func TestUpdate_ToggleURL_NoObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post",
			ObjectURI: "", // No ObjectURI
		},
	}
	m.Selected = 0

	// Try to toggle URL - should not work without ObjectURI
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if m.showingURL {
		t.Error("Expected showingURL false when no ObjectURI")
	}
}

func TestUpdate_ReplyToPost(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:    uuid.New(),
			Author:    "testuser",
			Content:   "Test post content",
			ObjectURI: "https://example.com/notes/1",
			IsLocal:   true,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	if replyMsg.NoteURI != "https://example.com/notes/1" {
		t.Errorf("Expected NoteURI 'https://example.com/notes/1', got '%s'", replyMsg.NoteURI)
	}
	if replyMsg.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", replyMsg.Author)
	}
}

func TestUpdate_ReplyToLocalPostWithoutObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:    noteID,
			Author:    "testuser",
			Content:   "Local post",
			ObjectURI: "", // No ObjectURI
			IsLocal:   true,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if cmd == nil {
		t.Fatal("Expected command for reply")
	}

	msg := cmd()
	replyMsg, ok := msg.(common.ReplyToNoteMsg)
	if !ok {
		t.Fatalf("Expected ReplyToNoteMsg, got %T", msg)
	}

	expectedURI := "local:" + noteID.String()
	if replyMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, replyMsg.NoteURI)
	}
}

func TestUpdate_EnterOnPostWithReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:     noteID,
			Author:     "testuser",
			Content:    "Post with replies",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 5,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for thread view")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	if viewMsg.NoteURI != "https://example.com/notes/1" {
		t.Errorf("Expected NoteURI, got '%s'", viewMsg.NoteURI)
	}
	if viewMsg.NoteID != noteID {
		t.Errorf("Expected NoteID %v, got %v", noteID, viewMsg.NoteID)
	}
	if !viewMsg.IsLocal {
		t.Error("Expected IsLocal true")
	}
}

func TestUpdate_EnterOnPostWithoutReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Post without replies",
			ObjectURI:  "https://example.com/notes/1",
			IsLocal:    true,
			ReplyCount: 0,
		},
	}
	m.Selected = 0

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd != nil {
		t.Error("Expected no command for post without replies")
	}
}

func TestUpdate_EnterOnLocalPostWithoutObjectURI(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	noteID := uuid.New()
	m.Posts = []domain.HomePost{
		{
			NoteID:     noteID,
			Author:     "testuser",
			Content:    "Local post",
			ObjectURI:  "", // No ObjectURI
			IsLocal:    true,
			ReplyCount: 2,
		},
	}
	m.Selected = 0

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cmd == nil {
		t.Fatal("Expected command for thread view")
	}

	msg := cmd()
	viewMsg, ok := msg.(common.ViewThreadMsg)
	if !ok {
		t.Fatalf("Expected ViewThreadMsg, got %T", msg)
	}

	expectedURI := "local:" + noteID.String()
	if viewMsg.NoteURI != expectedURI {
		t.Errorf("Expected NoteURI '%s', got '%s'", expectedURI, viewMsg.NoteURI)
	}
}

func TestUpdate_SelectionBoundsAfterReload(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Selected = 5 // Selected past current bounds
	m.isActive = true

	posts := []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1"},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2"},
	}

	m, _ = m.Update(postsLoadedMsg{posts: posts})

	// Selected should be clamped to valid range
	if m.Selected >= len(m.Posts) {
		t.Errorf("Selected %d out of bounds for %d posts", m.Selected, len(m.Posts))
	}
	if m.Selected != 1 {
		t.Errorf("Expected Selected clamped to 1, got %d", m.Selected)
	}
}

func TestView_EmptyPosts(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{}

	view := m.View()

	if !strings.Contains(view, "No posts yet") {
		t.Error("Expected 'No posts yet' message")
	}
	if !strings.Contains(view, "Follow some accounts") {
		t.Error("Expected follow suggestion")
	}
}

func TestView_PostCount(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{NoteID: uuid.New(), Author: "user1", Content: "Post 1", Time: time.Now()},
		{NoteID: uuid.New(), Author: "user2", Content: "Post 2", Time: time.Now()},
		{NoteID: uuid.New(), Author: "user3", Content: "Post 3", Time: time.Now()},
	}

	view := m.View()

	if !strings.Contains(view, "3 posts") {
		t.Error("Expected '3 posts' in header")
	}
}

func TestView_SingularReply(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test",
			Time:       time.Now(),
			ReplyCount: 1,
		},
	}

	view := m.View()

	if !strings.Contains(view, "1 reply") {
		t.Error("Expected singular '1 reply'")
	}
}

func TestView_PluralReplies(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{
		{
			NoteID:     uuid.New(),
			Author:     "testuser",
			Content:    "Test",
			Time:       time.Now(),
			ReplyCount: 5,
		},
	}

	view := m.View()

	if !strings.Contains(view, "5 replies") {
		t.Error("Expected plural '5 replies'")
	}
}

func TestFormatTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "just now",
			time:     now.Add(-30 * time.Second),
			expected: "just now",
		},
		{
			name:     "minutes ago",
			time:     now.Add(-15 * time.Minute),
			expected: "15m ago",
		},
		{
			name:     "hours ago",
			time:     now.Add(-5 * time.Hour),
			expected: "5h ago",
		},
		{
			name:     "days ago",
			time:     now.Add(-3 * 24 * time.Hour),
			expected: "3d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTime(tt.time)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestMin(t *testing.T) {
	if min(5, 10) != 5 {
		t.Error("min(5, 10) should be 5")
	}
	if min(10, 5) != 5 {
		t.Error("min(10, 5) should be 5")
	}
	if min(5, 5) != 5 {
		t.Error("min(5, 5) should be 5")
	}
}

func TestMax(t *testing.T) {
	if max(5, 10) != 10 {
		t.Error("max(5, 10) should be 10")
	}
	if max(10, 5) != 10 {
		t.Error("max(10, 5) should be 10")
	}
	if max(5, 5) != 5 {
		t.Error("max(5, 5) should be 5")
	}
}

func TestHomePost_Fields(t *testing.T) {
	noteID := uuid.New()
	now := time.Now()

	post := domain.HomePost{
		NoteID:     noteID,
		Author:     "testuser",
		Content:    "Test content",
		Time:       now,
		ObjectURI:  "https://example.com/notes/123",
		IsLocal:    true,
		ReplyCount: 3,
	}

	if post.NoteID != noteID {
		t.Errorf("Expected NoteID %v, got %v", noteID, post.NoteID)
	}
	if post.Author != "testuser" {
		t.Errorf("Expected Author 'testuser', got '%s'", post.Author)
	}
	if post.Content != "Test content" {
		t.Errorf("Expected Content 'Test content', got '%s'", post.Content)
	}
	if post.ReplyCount != 3 {
		t.Errorf("Expected ReplyCount 3, got %d", post.ReplyCount)
	}
	if !post.IsLocal {
		t.Error("Expected IsLocal true")
	}
}

func TestUpdate_EmptyPosts_NoCrash(t *testing.T) {
	m := InitialModel(uuid.New(), 120, 40, "")
	m.Posts = []domain.HomePost{}

	// Navigation on empty posts should not crash
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	// Should complete without panic
}

package common

import (
	"time"

	"github.com/google/uuid"
)

type SessionState uint

const (
	CreateNoteView SessionState = iota
	ListNotesView
	CreateUserView
	UpdateNoteList
	FollowUserView        // Follow remote users
	FollowersView         // View who follows you
	FollowingView         // View who you're following
	FederatedTimelineView // View federated posts
	LocalTimelineView     // View local posts from all local users
	LocalUsersView        // Browse and follow local users
	AdminPanelView        // Admin panel for user management (admin only)
	DeleteAccountView     // Delete account with confirmation
	ThreadView            // View thread with parent and replies
)

// EditNoteMsg is sent when user wants to edit an existing note
type EditNoteMsg struct {
	NoteId    uuid.UUID
	Message   string
	CreatedAt time.Time
}

// DeleteNoteMsg is sent when user confirms note deletion
type DeleteNoteMsg struct {
	NoteId uuid.UUID
}

// ActivateViewMsg is sent when a view becomes active (visible)
type ActivateViewMsg struct{}

// DeactivateViewMsg is sent when a view becomes inactive (hidden)
type DeactivateViewMsg struct{}

// ReplyToNoteMsg is sent when user presses 'r' to reply to a post
type ReplyToNoteMsg struct {
	NoteURI string // ActivityPub object URI of the note being replied to
	Author  string // Display name or handle of the author
	Preview string // Preview of the note content (first line or truncated)
}

// ViewThreadMsg is sent when user presses Enter to view a thread
type ViewThreadMsg struct {
	NoteURI   string    // ActivityPub object URI of the note
	NoteID    uuid.UUID // Local UUID (if local note)
	IsLocal   bool      // Whether this is a local note
	Author    string    // Author name for display
	Content   string    // Full content
	CreatedAt time.Time // Timestamp
}

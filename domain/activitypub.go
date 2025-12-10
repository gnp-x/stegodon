package domain

import (
	"github.com/google/uuid"
	"time"
)

// RemoteAccount represents a cached federated user
type RemoteAccount struct {
	Id            uuid.UUID
	Username      string
	Domain        string
	ActorURI      string
	DisplayName   string
	Summary       string
	InboxURI      string
	OutboxURI     string
	PublicKeyPem  string
	AvatarURL     string
	LastFetchedAt time.Time
}

// Follow represents a follow relationship
type Follow struct {
	Id              uuid.UUID
	AccountId       uuid.UUID // Can be local or remote account
	TargetAccountId uuid.UUID // Can be local or remote account
	URI             string    // ActivityPub Follow activity URI (empty for local follows)
	CreatedAt       time.Time
	Accepted        bool
	IsLocal         bool // true if this is a local-only follow
}

// Like represents a like/favorite on a note
type Like struct {
	Id        uuid.UUID
	AccountId uuid.UUID // Who liked (can be local or remote)
	NoteId    uuid.UUID // Which note was liked
	URI       string    // ActivityPub Like activity URI
	CreatedAt time.Time
}

// Boost represents a boost/reblog/announce on a note
type Boost struct {
	Id        uuid.UUID
	AccountId uuid.UUID // Who boosted (can be local or remote)
	NoteId    uuid.UUID // Which note was boosted
	URI       string    // ActivityPub Announce activity URI
	CreatedAt time.Time
}

// Activity represents an ActivityPub activity (for logging/deduplication)
type Activity struct {
	Id           uuid.UUID
	ActivityURI  string
	ActivityType string // Follow, Create, Like, Announce, Undo, etc.
	ActorURI     string
	ObjectURI    string
	RawJSON      string
	Processed    bool
	CreatedAt    time.Time
	Local        bool // true if originated from this server
	FromRelay    bool // true if forwarded by a relay
	LikeCount    int  // Denormalized like count
	BoostCount   int  // Denormalized boost count
}

// DeliveryQueueItem represents an item in the delivery queue
type DeliveryQueueItem struct {
	Id           uuid.UUID
	InboxURI     string
	ActivityJSON string // The complete activity to deliver
	Attempts     int
	NextRetryAt  time.Time
	CreatedAt    time.Time
}

// NoteMention represents a @user@domain mention in a note
type NoteMention struct {
	Id                uuid.UUID
	NoteId            uuid.UUID
	MentionedActorURI string // The ActivityPub actor URI of the mentioned user
	MentionedUsername string // The username part (@username@domain -> username)
	MentionedDomain   string // The domain part (@username@domain -> domain)
	CreatedAt         time.Time
}

// Relay represents an ActivityPub relay subscription
type Relay struct {
	Id         uuid.UUID
	ActorURI   string // The relay's actor URI (e.g., https://relay.example.com/actor)
	InboxURI   string // The relay's inbox URI for delivering activities
	FollowURI  string // The URI of our Follow activity (needed for Undo)
	Name       string // Display name from relay actor profile
	Status     string // pending, active, failed
	CreatedAt  time.Time
	AcceptedAt *time.Time // When the relay accepted our Follow request
}

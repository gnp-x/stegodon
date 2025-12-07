package activitypub

import (
	"net/http"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// Database defines the database operations required by the ActivityPub package.
// This interface allows for dependency injection and testing with mock implementations.
type Database interface {
	// Account operations
	ReadAccByUsername(username string) (error, *domain.Account)
	ReadAccById(id uuid.UUID) (error, *domain.Account)

	// Remote account operations
	ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount)
	ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount)
	ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount)
	CreateRemoteAccount(acc *domain.RemoteAccount) error
	UpdateRemoteAccount(acc *domain.RemoteAccount) error
	DeleteRemoteAccount(id uuid.UUID) error

	// Follow operations
	CreateFollow(follow *domain.Follow) error
	ReadFollowByURI(uri string) (error, *domain.Follow)
	ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow)
	DeleteFollowByURI(uri string) error
	AcceptFollowByURI(uri string) error
	ReadFollowersByAccountId(accountId uuid.UUID) (error, *[]domain.Follow)
	DeleteFollowsByRemoteAccountId(remoteAccountId uuid.UUID) error

	// Activity operations
	CreateActivity(activity *domain.Activity) error
	UpdateActivity(activity *domain.Activity) error
	ReadActivityByObjectURI(objectURI string) (error, *domain.Activity)
	DeleteActivity(id uuid.UUID) error

	// Note operations (for replies)
	ReadNoteByURI(objectURI string) (error, *domain.Note)

	// Mention operations
	CreateNoteMention(mention *domain.NoteMention) error

	// Engagement count operations
	IncrementReplyCountByURI(parentURI string) error

	// Delivery queue operations
	EnqueueDelivery(item *domain.DeliveryQueueItem) error
	ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem)
	UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error
	DeleteDelivery(id uuid.UUID) error
}

// HTTPClient defines the HTTP client operations required by the ActivityPub package.
// This interface allows for dependency injection and testing with mock implementations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// DefaultHTTPClient is the default HTTP client used in production
type DefaultHTTPClient struct {
	client *http.Client
}

// NewDefaultHTTPClient creates a new default HTTP client with the specified timeout
func NewDefaultHTTPClient(timeout time.Duration) *DefaultHTTPClient {
	return &DefaultHTTPClient{
		client: &http.Client{Timeout: timeout},
	}
}

// Do executes the HTTP request
func (c *DefaultHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

package activitypub

import (
	"time"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// DBWrapper wraps the real database to implement the Database interface.
// This adapter allows the production code to use the existing db.GetDB() singleton
// while also supporting dependency injection for tests.
type DBWrapper struct {
	db *db.DB
}

// NewDBWrapper creates a new database wrapper around the singleton database
func NewDBWrapper() *DBWrapper {
	return &DBWrapper{db: db.GetDB()}
}

// Account operations

func (w *DBWrapper) ReadAccByUsername(username string) (error, *domain.Account) {
	return w.db.ReadAccByUsername(username)
}

func (w *DBWrapper) ReadAccById(id uuid.UUID) (error, *domain.Account) {
	return w.db.ReadAccById(id)
}

// Remote account operations

func (w *DBWrapper) ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount) {
	return w.db.ReadRemoteAccountByURI(uri)
}

func (w *DBWrapper) ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount) {
	return w.db.ReadRemoteAccountById(id)
}

func (w *DBWrapper) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	return w.db.ReadRemoteAccountByActorURI(actorURI)
}

func (w *DBWrapper) CreateRemoteAccount(acc *domain.RemoteAccount) error {
	return w.db.CreateRemoteAccount(acc)
}

func (w *DBWrapper) UpdateRemoteAccount(acc *domain.RemoteAccount) error {
	return w.db.UpdateRemoteAccount(acc)
}

func (w *DBWrapper) DeleteRemoteAccount(id uuid.UUID) error {
	return w.db.DeleteRemoteAccount(id)
}

// Follow operations

func (w *DBWrapper) CreateFollow(follow *domain.Follow) error {
	return w.db.CreateFollow(follow)
}

func (w *DBWrapper) ReadFollowByURI(uri string) (error, *domain.Follow) {
	return w.db.ReadFollowByURI(uri)
}

func (w *DBWrapper) ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow) {
	return w.db.ReadFollowByAccountIds(accountId, targetAccountId)
}

func (w *DBWrapper) DeleteFollowByURI(uri string) error {
	return w.db.DeleteFollowByURI(uri)
}

func (w *DBWrapper) AcceptFollowByURI(uri string) error {
	return w.db.AcceptFollowByURI(uri)
}

func (w *DBWrapper) ReadFollowersByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	return w.db.ReadFollowersByAccountId(accountId)
}

func (w *DBWrapper) DeleteFollowsByRemoteAccountId(remoteAccountId uuid.UUID) error {
	return w.db.DeleteFollowsByRemoteAccountId(remoteAccountId)
}

// Activity operations

func (w *DBWrapper) CreateActivity(activity *domain.Activity) error {
	return w.db.CreateActivity(activity)
}

func (w *DBWrapper) UpdateActivity(activity *domain.Activity) error {
	return w.db.UpdateActivity(activity)
}

func (w *DBWrapper) ReadActivityByURI(uri string) (error, *domain.Activity) {
	return w.db.ReadActivityByURI(uri)
}

func (w *DBWrapper) ReadActivityByObjectURI(objectURI string) (error, *domain.Activity) {
	return w.db.ReadActivityByObjectURI(objectURI)
}

func (w *DBWrapper) DeleteActivity(id uuid.UUID) error {
	return w.db.DeleteActivity(id)
}

// Note operations

func (w *DBWrapper) ReadNoteByURI(objectURI string) (error, *domain.Note) {
	return w.db.ReadNoteByURI(objectURI)
}

// Mention operations

func (w *DBWrapper) CreateNoteMention(mention *domain.NoteMention) error {
	return w.db.CreateNoteMention(mention)
}

// Engagement count operations

func (w *DBWrapper) IncrementReplyCountByURI(parentURI string) error {
	return w.db.IncrementReplyCountByURI(parentURI)
}

// Like operations

func (w *DBWrapper) CreateLike(like *domain.Like) error {
	return w.db.CreateLike(like)
}

func (w *DBWrapper) HasLikeByURI(uri string) (bool, error) {
	return w.db.HasLikeByURI(uri)
}

func (w *DBWrapper) HasLike(accountId, noteId uuid.UUID) (bool, error) {
	return w.db.HasLike(accountId, noteId)
}

func (w *DBWrapper) ReadLikeByAccountAndNote(accountId, noteId uuid.UUID) (error, *domain.Like) {
	return w.db.ReadLikeByAccountAndNote(accountId, noteId)
}

func (w *DBWrapper) DeleteLikeByAccountAndNote(accountId, noteId uuid.UUID) error {
	return w.db.DeleteLikeByAccountAndNote(accountId, noteId)
}

func (w *DBWrapper) IncrementLikeCountByNoteId(noteId uuid.UUID) error {
	return w.db.IncrementLikeCountByNoteId(noteId)
}

func (w *DBWrapper) DecrementLikeCountByNoteId(noteId uuid.UUID) error {
	return w.db.DecrementLikeCountByNoteId(noteId)
}

// Boost operations

func (w *DBWrapper) CreateBoost(boost *domain.Boost) error {
	return w.db.CreateBoost(boost)
}

func (w *DBWrapper) HasBoost(accountId, noteId uuid.UUID) (bool, error) {
	return w.db.HasBoost(accountId, noteId)
}

func (w *DBWrapper) DeleteBoostByAccountAndNote(accountId, noteId uuid.UUID) error {
	return w.db.DeleteBoostByAccountAndNote(accountId, noteId)
}

func (w *DBWrapper) IncrementBoostCountByNoteId(noteId uuid.UUID) error {
	return w.db.IncrementBoostCountByNoteId(noteId)
}

func (w *DBWrapper) DecrementBoostCountByNoteId(noteId uuid.UUID) error {
	return w.db.DecrementBoostCountByNoteId(noteId)
}

// Delivery queue operations

func (w *DBWrapper) EnqueueDelivery(item *domain.DeliveryQueueItem) error {
	return w.db.EnqueueDelivery(item)
}

func (w *DBWrapper) ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem) {
	return w.db.ReadPendingDeliveries(limit)
}

func (w *DBWrapper) UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error {
	return w.db.UpdateDeliveryAttempt(id, attempts, nextRetry)
}

func (w *DBWrapper) DeleteDelivery(id uuid.UUID) error {
	return w.db.DeleteDelivery(id)
}

// Relay operations

func (w *DBWrapper) CreateRelay(relay *domain.Relay) error {
	return w.db.CreateRelay(relay)
}

func (w *DBWrapper) ReadActiveRelays() (error, *[]domain.Relay) {
	return w.db.ReadActiveRelays()
}

func (w *DBWrapper) ReadActiveUnpausedRelays() (error, *[]domain.Relay) {
	return w.db.ReadActiveUnpausedRelays()
}

func (w *DBWrapper) ReadRelayByActorURI(actorURI string) (error, *domain.Relay) {
	return w.db.ReadRelayByActorURI(actorURI)
}

func (w *DBWrapper) UpdateRelayStatus(id uuid.UUID, status string, acceptedAt *time.Time) error {
	return w.db.UpdateRelayStatus(id, status, acceptedAt)
}

func (w *DBWrapper) DeleteRelay(id uuid.UUID) error {
	return w.db.DeleteRelay(id)
}

// Notification operations

func (w *DBWrapper) CreateNotification(notification *domain.Notification) error {
	return w.db.CreateNotification(notification)
}

// Ensure DBWrapper implements Database interface
var _ Database = (*DBWrapper)(nil)

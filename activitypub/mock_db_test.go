package activitypub

import (
	"database/sql"
	"sync"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
)

// MockDatabase is an in-memory mock implementation of the Database interface for testing.
// It stores data in maps and provides full CRUD operations without requiring a real database.
type MockDatabase struct {
	mu sync.RWMutex

	// Storage maps
	Accounts        map[uuid.UUID]*domain.Account
	AccountsByUser  map[string]*domain.Account
	RemoteAccounts  map[uuid.UUID]*domain.RemoteAccount
	RemoteByURI     map[string]*domain.RemoteAccount
	RemoteByActor   map[string]*domain.RemoteAccount
	Follows         map[uuid.UUID]*domain.Follow
	FollowsByURI    map[string]*domain.Follow
	Activities      map[uuid.UUID]*domain.Activity
	ActivitiesByObj map[string]*domain.Activity
	DeliveryQueue   map[uuid.UUID]*domain.DeliveryQueueItem
	Notes           map[uuid.UUID]*domain.Note
	NotesByURI      map[string]*domain.Note

	// Error injection for testing error handling
	ForceError error
}

// NewMockDatabase creates a new mock database with initialized maps
func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		Accounts:        make(map[uuid.UUID]*domain.Account),
		AccountsByUser:  make(map[string]*domain.Account),
		RemoteAccounts:  make(map[uuid.UUID]*domain.RemoteAccount),
		RemoteByURI:     make(map[string]*domain.RemoteAccount),
		RemoteByActor:   make(map[string]*domain.RemoteAccount),
		Follows:         make(map[uuid.UUID]*domain.Follow),
		FollowsByURI:    make(map[string]*domain.Follow),
		Activities:      make(map[uuid.UUID]*domain.Activity),
		ActivitiesByObj: make(map[string]*domain.Activity),
		DeliveryQueue:   make(map[uuid.UUID]*domain.DeliveryQueueItem),
		Notes:           make(map[uuid.UUID]*domain.Note),
		NotesByURI:      make(map[string]*domain.Note),
	}
}

// SetForceError sets an error to be returned by all operations
func (m *MockDatabase) SetForceError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ForceError = err
}

// AddAccount adds an account to the mock database
func (m *MockDatabase) AddAccount(acc *domain.Account) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Accounts[acc.Id] = acc
	m.AccountsByUser[acc.Username] = acc
}

// AddRemoteAccount adds a remote account to the mock database
func (m *MockDatabase) AddRemoteAccount(acc *domain.RemoteAccount) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
}

// AddFollow adds a follow relationship to the mock database
func (m *MockDatabase) AddFollow(follow *domain.Follow) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Follows[follow.Id] = follow
	if follow.URI != "" {
		m.FollowsByURI[follow.URI] = follow
	}
}

// AddActivity adds an activity to the mock database
func (m *MockDatabase) AddActivity(activity *domain.Activity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Activities[activity.Id] = activity
	if activity.ObjectURI != "" {
		m.ActivitiesByObj[activity.ObjectURI] = activity
	}
}

// AddDeliveryQueueItem adds a delivery queue item to the mock database
func (m *MockDatabase) AddDeliveryQueueItem(item *domain.DeliveryQueueItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeliveryQueue[item.Id] = item
}

// Account operations

func (m *MockDatabase) ReadAccByUsername(username string) (error, *domain.Account) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.AccountsByUser[username]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadAccById(id uuid.UUID) (error, *domain.Account) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.Accounts[id]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

// Remote account operations

func (m *MockDatabase) ReadRemoteAccountByURI(uri string) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteByURI[uri]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadRemoteAccountById(id uuid.UUID) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteAccounts[id]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, acc
}

func (m *MockDatabase) ReadRemoteAccountByActorURI(actorURI string) (error, *domain.RemoteAccount) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	acc, ok := m.RemoteByActor[actorURI]
	if !ok {
		return nil, nil
	}
	return nil, acc
}

func (m *MockDatabase) CreateRemoteAccount(acc *domain.RemoteAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
	return nil
}

func (m *MockDatabase) UpdateRemoteAccount(acc *domain.RemoteAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.RemoteAccounts[acc.Id] = acc
	m.RemoteByURI[acc.ActorURI] = acc
	m.RemoteByActor[acc.ActorURI] = acc
	return nil
}

func (m *MockDatabase) DeleteRemoteAccount(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if acc, ok := m.RemoteAccounts[id]; ok {
		delete(m.RemoteByURI, acc.ActorURI)
		delete(m.RemoteByActor, acc.ActorURI)
	}
	delete(m.RemoteAccounts, id)
	return nil
}

// Follow operations

func (m *MockDatabase) CreateFollow(follow *domain.Follow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Follows[follow.Id] = follow
	if follow.URI != "" {
		m.FollowsByURI[follow.URI] = follow
	}
	return nil
}

func (m *MockDatabase) ReadFollowByURI(uri string) (error, *domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	follow, ok := m.FollowsByURI[uri]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, follow
}

func (m *MockDatabase) ReadFollowByAccountIds(accountId, targetAccountId uuid.UUID) (error, *domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	for _, follow := range m.Follows {
		if follow.AccountId == accountId && follow.TargetAccountId == targetAccountId {
			return nil, follow
		}
	}
	return sql.ErrNoRows, nil
}

func (m *MockDatabase) DeleteFollowByURI(uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if follow, ok := m.FollowsByURI[uri]; ok {
		delete(m.Follows, follow.Id)
	}
	delete(m.FollowsByURI, uri)
	return nil
}

func (m *MockDatabase) AcceptFollowByURI(uri string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if follow, ok := m.FollowsByURI[uri]; ok {
		follow.Accepted = true
	}
	return nil
}

func (m *MockDatabase) ReadFollowersByAccountId(accountId uuid.UUID) (error, *[]domain.Follow) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	var followers []domain.Follow
	for _, follow := range m.Follows {
		if follow.TargetAccountId == accountId && follow.Accepted {
			followers = append(followers, *follow)
		}
	}
	return nil, &followers
}

func (m *MockDatabase) DeleteFollowsByRemoteAccountId(remoteAccountId uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	for id, follow := range m.Follows {
		if follow.AccountId == remoteAccountId || follow.TargetAccountId == remoteAccountId {
			if follow.URI != "" {
				delete(m.FollowsByURI, follow.URI)
			}
			delete(m.Follows, id)
		}
	}
	return nil
}

// Activity operations

func (m *MockDatabase) CreateActivity(activity *domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Activities[activity.Id] = activity
	if activity.ObjectURI != "" {
		// Only set if not already present (first activity with this ObjectURI wins)
		// This matches real DB behavior where ReadActivityByObjectURI returns the first match
		if _, exists := m.ActivitiesByObj[activity.ObjectURI]; !exists {
			m.ActivitiesByObj[activity.ObjectURI] = activity
		}
	}
	return nil
}

func (m *MockDatabase) UpdateActivity(activity *domain.Activity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.Activities[activity.Id] = activity
	if activity.ObjectURI != "" {
		m.ActivitiesByObj[activity.ObjectURI] = activity
	}
	return nil
}

func (m *MockDatabase) ReadActivityByObjectURI(objectURI string) (error, *domain.Activity) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	activity, ok := m.ActivitiesByObj[objectURI]
	if !ok {
		return nil, nil
	}
	return nil, activity
}

func (m *MockDatabase) DeleteActivity(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if activity, ok := m.Activities[id]; ok {
		delete(m.ActivitiesByObj, activity.ObjectURI)
	}
	delete(m.Activities, id)
	return nil
}

// Delivery queue operations

func (m *MockDatabase) EnqueueDelivery(item *domain.DeliveryQueueItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	m.DeliveryQueue[item.Id] = item
	return nil
}

func (m *MockDatabase) ReadPendingDeliveries(limit int) (error, *[]domain.DeliveryQueueItem) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	var items []domain.DeliveryQueueItem
	now := time.Now()
	count := 0
	for _, item := range m.DeliveryQueue {
		if item.NextRetryAt.Before(now) || item.NextRetryAt.Equal(now) {
			items = append(items, *item)
			count++
			if count >= limit {
				break
			}
		}
	}
	return nil, &items
}

func (m *MockDatabase) UpdateDeliveryAttempt(id uuid.UUID, attempts int, nextRetry time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	if item, ok := m.DeliveryQueue[id]; ok {
		item.Attempts = attempts
		item.NextRetryAt = nextRetry
	}
	return nil
}

func (m *MockDatabase) DeleteDelivery(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ForceError != nil {
		return m.ForceError
	}
	delete(m.DeliveryQueue, id)
	return nil
}

// Note operations

func (m *MockDatabase) ReadNoteByURI(objectURI string) (error, *domain.Note) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.ForceError != nil {
		return m.ForceError, nil
	}
	note, ok := m.NotesByURI[objectURI]
	if !ok {
		return sql.ErrNoRows, nil
	}
	return nil, note
}

// AddNote adds a note to the mock database
func (m *MockDatabase) AddNote(note *domain.Note) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Notes[note.Id] = note
	if note.ObjectURI != "" {
		m.NotesByURI[note.ObjectURI] = note
	}
}

// Ensure MockDatabase implements Database interface
var _ Database = (*MockDatabase)(nil)

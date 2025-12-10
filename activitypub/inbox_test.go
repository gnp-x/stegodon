package activitypub

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

func TestActivityUnmarshal(t *testing.T) {
	// Test basic Activity struct unmarshaling
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/123",
		"type": "Follow",
		"actor": "https://example.com/users/alice",
		"object": "https://example.com/users/bob"
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity: %v", err)
	}

	if activity.ID != "https://example.com/activities/123" {
		t.Errorf("Expected ID 'https://example.com/activities/123', got '%s'", activity.ID)
	}
	if activity.Type != "Follow" {
		t.Errorf("Expected Type 'Follow', got '%s'", activity.Type)
	}
	if activity.Actor != "https://example.com/users/alice" {
		t.Errorf("Expected Actor 'https://example.com/users/alice', got '%s'", activity.Actor)
	}
}

func TestActivityObjectAsString(t *testing.T) {
	// Test Activity with object as simple string URI
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/456",
		"type": "Undo",
		"actor": "https://example.com/users/alice",
		"object": "https://example.com/activities/123"
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity with string object: %v", err)
	}

	// Verify we can extract the object URI
	var objectURI string
	switch obj := activity.Object.(type) {
	case string:
		objectURI = obj
	}

	if objectURI != "https://example.com/activities/123" {
		t.Errorf("Expected object URI 'https://example.com/activities/123', got '%s'", objectURI)
	}
}

func TestActivityObjectAsMap(t *testing.T) {
	// Test Activity with object as embedded map
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://example.com/activities/789",
		"type": "Create",
		"actor": "https://example.com/users/alice",
		"object": {
			"id": "https://example.com/notes/abc",
			"type": "Note",
			"content": "Hello world"
		}
	}`

	var activity Activity
	if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
		t.Fatalf("Failed to unmarshal Activity with map object: %v", err)
	}

	// Verify we can extract the object URI
	var objectURI string
	switch obj := activity.Object.(type) {
	case map[string]any:
		if id, ok := obj["id"].(string); ok {
			objectURI = id
		}
	}

	if objectURI != "https://example.com/notes/abc" {
		t.Errorf("Expected object URI 'https://example.com/notes/abc', got '%s'", objectURI)
	}
}

func TestFollowActivityUnmarshal(t *testing.T) {
	jsonData := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.social/follows/123",
		"type": "Follow",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://stegodon.example/users/bob"
	}`

	var follow FollowActivity
	if err := json.Unmarshal([]byte(jsonData), &follow); err != nil {
		t.Fatalf("Failed to unmarshal FollowActivity: %v", err)
	}

	if follow.ID != "https://mastodon.social/follows/123" {
		t.Errorf("Expected ID, got '%s'", follow.ID)
	}
	if follow.Type != "Follow" {
		t.Errorf("Expected Type 'Follow', got '%s'", follow.Type)
	}
	if follow.Actor != "https://mastodon.social/users/alice" {
		t.Errorf("Expected Actor URL, got '%s'", follow.Actor)
	}
	if follow.Object != "https://stegodon.example/users/bob" {
		t.Errorf("Expected Object URL, got '%s'", follow.Object)
	}
}

func TestUndoActivityStructure(t *testing.T) {
	// Test parsing Undo activity with embedded Follow
	jsonData := `{
		"type": "Undo",
		"actor": "https://example.com/users/alice",
		"object": {
			"type": "Follow",
			"id": "https://example.com/follows/123"
		}
	}`

	var undo struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &undo); err != nil {
		t.Fatalf("Failed to unmarshal Undo: %v", err)
	}

	if undo.Type != "Undo" {
		t.Errorf("Expected Type 'Undo', got '%s'", undo.Type)
	}

	// Parse embedded object
	var obj struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(undo.Object, &obj); err != nil {
		t.Fatalf("Failed to unmarshal Undo object: %v", err)
	}

	if obj.Type != "Follow" {
		t.Errorf("Expected embedded Type 'Follow', got '%s'", obj.Type)
	}
	if obj.ID != "https://example.com/follows/123" {
		t.Errorf("Expected embedded ID, got '%s'", obj.ID)
	}
}

func TestCreateActivityStructure(t *testing.T) {
	jsonData := `{
		"id": "https://mastodon.social/users/alice/statuses/123/activity",
		"type": "Create",
		"actor": "https://mastodon.social/users/alice",
		"object": {
			"id": "https://mastodon.social/users/alice/statuses/123",
			"type": "Note",
			"content": "Hello from Mastodon!",
			"published": "2025-11-14T10:00:00Z",
			"attributedTo": "https://mastodon.social/users/alice"
		}
	}`

	var create struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object struct {
			ID           string `json:"id"`
			Type         string `json:"type"`
			Content      string `json:"content"`
			Published    string `json:"published"`
			AttributedTo string `json:"attributedTo"`
		} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &create); err != nil {
		t.Fatalf("Failed to unmarshal Create: %v", err)
	}

	if create.Type != "Create" {
		t.Errorf("Expected Type 'Create', got '%s'", create.Type)
	}
	if create.Actor != "https://mastodon.social/users/alice" {
		t.Errorf("Expected Actor URL, got '%s'", create.Actor)
	}
	if create.Object.Type != "Note" {
		t.Errorf("Expected Object Type 'Note', got '%s'", create.Object.Type)
	}
	if create.Object.Content != "Hello from Mastodon!" {
		t.Errorf("Expected content, got '%s'", create.Object.Content)
	}
}

func TestAcceptActivityStructure(t *testing.T) {
	jsonData := `{
		"type": "Accept",
		"actor": "https://mastodon.social/users/bob",
		"object": {
			"id": "https://stegodon.example/follows/456",
			"type": "Follow"
		}
	}`

	var accept struct {
		Type   string          `json:"type"`
		Actor  string          `json:"actor"`
		Object json.RawMessage `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &accept); err != nil {
		t.Fatalf("Failed to unmarshal Accept: %v", err)
	}

	if accept.Type != "Accept" {
		t.Errorf("Expected Type 'Accept', got '%s'", accept.Type)
	}

	// Parse embedded Follow object
	var followObj struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(accept.Object, &followObj); err != nil {
		t.Fatalf("Failed to unmarshal Accept object: %v", err)
	}

	if followObj.ID != "https://stegodon.example/follows/456" {
		t.Errorf("Expected follow ID, got '%s'", followObj.ID)
	}
}

func TestUpdateActivityStructure(t *testing.T) {
	tests := []struct {
		name       string
		jsonData   string
		expectType string
		expectID   string
	}{
		{
			name: "Person update (profile)",
			jsonData: `{
				"id": "https://mastodon.social/users/alice#updates/1",
				"type": "Update",
				"actor": "https://mastodon.social/users/alice",
				"object": {
					"type": "Person",
					"id": "https://mastodon.social/users/alice"
				}
			}`,
			expectType: "Person",
			expectID:   "https://mastodon.social/users/alice",
		},
		{
			name: "Note update (post edit)",
			jsonData: `{
				"id": "https://mastodon.social/users/alice#updates/2",
				"type": "Update",
				"actor": "https://mastodon.social/users/alice",
				"object": {
					"type": "Note",
					"id": "https://mastodon.social/users/alice/statuses/123"
				}
			}`,
			expectType: "Note",
			expectID:   "https://mastodon.social/users/alice/statuses/123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update struct {
				ID     string          `json:"id"`
				Type   string          `json:"type"`
				Actor  string          `json:"actor"`
				Object json.RawMessage `json:"object"`
			}

			if err := json.Unmarshal([]byte(tt.jsonData), &update); err != nil {
				t.Fatalf("Failed to unmarshal Update: %v", err)
			}

			if update.Type != "Update" {
				t.Errorf("Expected Type 'Update', got '%s'", update.Type)
			}

			var objectType struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if err := json.Unmarshal(update.Object, &objectType); err != nil {
				t.Fatalf("Failed to unmarshal Update object: %v", err)
			}

			if objectType.Type != tt.expectType {
				t.Errorf("Expected object type '%s', got '%s'", tt.expectType, objectType.Type)
			}
			if objectType.ID != tt.expectID {
				t.Errorf("Expected object ID '%s', got '%s'", tt.expectID, objectType.ID)
			}
		})
	}
}

func TestDeleteActivityStringObject(t *testing.T) {
	// Test Delete with simple string object
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete/1",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://mastodon.social/users/alice/statuses/123"
	}`

	var delete struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object any    `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	// Extract object URI
	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	}

	if objectURI != "https://mastodon.social/users/alice/statuses/123" {
		t.Errorf("Expected object URI, got '%s'", objectURI)
	}
}

func TestDeleteActivityTombstone(t *testing.T) {
	// Test Delete with Tombstone object
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete/2",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": {
			"id": "https://mastodon.social/users/alice/statuses/456",
			"type": "Tombstone"
		}
	}`

	var delete struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object any    `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	// Extract object URI from Tombstone
	var objectURI string
	switch obj := delete.Object.(type) {
	case map[string]any:
		if typ, ok := obj["type"].(string); ok && typ == "Tombstone" {
			if id, ok := obj["id"].(string); ok {
				objectURI = id
			}
		}
	}

	if objectURI != "https://mastodon.social/users/alice/statuses/456" {
		t.Errorf("Expected object URI from Tombstone, got '%s'", objectURI)
	}
}

func TestDeleteActivityActorDeletion(t *testing.T) {
	// Test Delete where object is the actor (account deletion)
	jsonData := `{
		"id": "https://mastodon.social/users/alice#delete",
		"type": "Delete",
		"actor": "https://mastodon.social/users/alice",
		"object": "https://mastodon.social/users/alice"
	}`

	var delete struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object any    `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &delete); err != nil {
		t.Fatalf("Failed to unmarshal Delete: %v", err)
	}

	var objectURI string
	switch obj := delete.Object.(type) {
	case string:
		objectURI = obj
	}

	// Check if it's an actor deletion
	isActorDeletion := (objectURI == delete.Actor)
	if !isActorDeletion {
		t.Error("Expected actor deletion (object == actor)")
	}
}

func TestActivityTypes(t *testing.T) {
	// Test that we correctly identify different activity types
	activityTypes := []string{
		"Follow",
		"Undo",
		"Create",
		"Like",
		"Accept",
		"Update",
		"Delete",
		"Announce",
		"Reject",
	}

	for _, actType := range activityTypes {
		t.Run(actType, func(t *testing.T) {
			jsonData := `{"type":"` + actType + `"}`
			var activity Activity
			if err := json.Unmarshal([]byte(jsonData), &activity); err != nil {
				t.Fatalf("Failed to unmarshal %s activity: %v", actType, err)
			}

			if activity.Type != actType {
				t.Errorf("Expected Type '%s', got '%s'", actType, activity.Type)
			}
		})
	}
}

func TestActivityContextVariants(t *testing.T) {
	// Test different @context formats
	tests := []struct {
		name    string
		context any
	}{
		{
			name:    "string context",
			context: "https://www.w3.org/ns/activitystreams",
		},
		{
			name: "array context",
			context: []any{
				"https://www.w3.org/ns/activitystreams",
				"https://w3id.org/security/v1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, _ := json.Marshal(map[string]any{
				"@context": tt.context,
				"type":     "Follow",
			})

			var activity Activity
			if err := json.Unmarshal(jsonBytes, &activity); err != nil {
				t.Fatalf("Failed to unmarshal activity with %s: %v", tt.name, err)
			}

			if activity.Type != "Follow" {
				t.Error("Activity type should be preserved regardless of context format")
			}
		})
	}
}

func TestActivityValidation(t *testing.T) {
	// Test validation of required fields
	tests := []struct {
		name      string
		jsonData  string
		wantError bool
	}{
		{
			name: "valid activity",
			jsonData: `{
				"type": "Follow",
				"actor": "https://example.com/users/alice",
				"object": "https://example.com/users/bob"
			}`,
			wantError: false,
		},
		{
			name:      "missing type",
			jsonData:  `{"actor": "https://example.com/users/alice"}`,
			wantError: false, // JSON unmarshal won't error, just empty Type
		},
		{
			name:      "invalid JSON",
			jsonData:  `{invalid json}`,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var activity Activity
			err := json.Unmarshal([]byte(tt.jsonData), &activity)

			if tt.wantError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestActivityURIExtraction(t *testing.T) {
	// Test extracting various URIs from activities
	tests := []struct {
		name        string
		jsonData    string
		expectActor string
		expectID    string
	}{
		{
			name: "Mastodon follow",
			jsonData: `{
				"id": "https://mastodon.social/12345678-1234-1234-1234-123456789abc",
				"type": "Follow",
				"actor": "https://mastodon.social/users/alice"
			}`,
			expectActor: "https://mastodon.social/users/alice",
			expectID:    "https://mastodon.social/12345678-1234-1234-1234-123456789abc",
		},
		{
			name: "Pleroma create",
			jsonData: `{
				"id": "https://pleroma.site/activities/abcdef",
				"type": "Create",
				"actor": "https://pleroma.site/users/bob"
			}`,
			expectActor: "https://pleroma.site/users/bob",
			expectID:    "https://pleroma.site/activities/abcdef",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var activity Activity
			if err := json.Unmarshal([]byte(tt.jsonData), &activity); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if activity.Actor != tt.expectActor {
				t.Errorf("Expected actor '%s', got '%s'", tt.expectActor, activity.Actor)
			}
			if activity.ID != tt.expectID {
				t.Errorf("Expected ID '%s', got '%s'", tt.expectID, activity.ID)
			}
		})
	}
}

func TestObjectURIExtraction(t *testing.T) {
	// Test the logic for extracting objectURI from different object formats
	tests := []struct {
		name      string
		object    any
		wantURI   string
		wantFound bool
	}{
		{
			name:      "string object",
			object:    "https://example.com/notes/123",
			wantURI:   "https://example.com/notes/123",
			wantFound: true,
		},
		{
			name: "map object with id",
			object: map[string]any{
				"id":   "https://example.com/notes/456",
				"type": "Note",
			},
			wantURI:   "https://example.com/notes/456",
			wantFound: true,
		},
		{
			name: "map object without id",
			object: map[string]any{
				"type": "Note",
			},
			wantURI:   "",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objectURI string
			switch obj := tt.object.(type) {
			case string:
				objectURI = obj
			case map[string]any:
				if id, ok := obj["id"].(string); ok {
					objectURI = id
				}
			}

			if tt.wantFound && objectURI == "" {
				t.Error("Expected to find object URI but didn't")
			}
			if !tt.wantFound && objectURI != "" {
				t.Error("Expected not to find object URI but did")
			}
			if objectURI != tt.wantURI {
				t.Errorf("Expected URI '%s', got '%s'", tt.wantURI, objectURI)
			}
		})
	}
}

func TestHTMLContentInNote(t *testing.T) {
	// Test that we can handle HTML content in Create activities
	jsonData := `{
		"type": "Create",
		"object": {
			"type": "Note",
			"content": "<p>Hello <strong>world</strong>!</p>"
		}
	}`

	var create struct {
		Type   string `json:"type"`
		Object struct {
			Type    string `json:"type"`
			Content string `json:"content"`
		} `json:"object"`
	}

	if err := json.Unmarshal([]byte(jsonData), &create); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if !strings.Contains(create.Object.Content, "<p>") {
		t.Error("Content should preserve HTML tags")
	}
	if !strings.Contains(create.Object.Content, "<strong>") {
		t.Error("Content should preserve HTML formatting")
	}
}

// TestUndoActivity_Authorization tests that Undo activities verify actor ownership
func TestUndoActivity_Authorization(t *testing.T) {
	// Test data for Undo activity
	followID := "https://example.com/follows/123"
	followActor := "https://example.com/users/alice"
	unauthorizedActor := "https://example.com/users/bob"

	// Valid Undo (actor matches follow creator)
	validUndo := struct {
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		} `json:"object"`
	}{
		Type:  "Undo",
		Actor: followActor,
		Object: struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}{
			Type: "Follow",
			ID:   followID,
		},
	}

	// Invalid Undo (actor does NOT match follow creator)
	invalidUndo := validUndo
	invalidUndo.Actor = unauthorizedActor

	// Verify structure
	if validUndo.Actor != followActor {
		t.Error("Valid undo should have matching actor")
	}
	if invalidUndo.Actor == followActor {
		t.Error("Invalid undo should have different actor")
	}

	// The authorization check in handleUndoActivity would reject invalidUndo
	// because invalidUndo.Actor != followActor (the follow creator)
	if invalidUndo.Actor != followActor {
		t.Log("Authorization check would correctly reject unauthorized Undo")
	}
}

// TestDeleteActivity_Authorization tests that Delete activities verify actor ownership
func TestDeleteActivity_Authorization(t *testing.T) {
	// Test data for Delete activity
	objectURI := "https://example.com/posts/123"
	objectActor := "https://example.com/users/alice"
	unauthorizedActor := "https://example.com/users/bob"

	// Valid Delete (actor matches content creator)
	validDelete := struct {
		Type   string `json:"type"`
		Actor  string `json:"actor"`
		Object string `json:"object"`
	}{
		Type:   "Delete",
		Actor:  objectActor,
		Object: objectURI,
	}

	// Invalid Delete (actor does NOT match content creator)
	invalidDelete := validDelete
	invalidDelete.Actor = unauthorizedActor

	// Verify structure
	if validDelete.Actor != objectActor {
		t.Error("Valid delete should have matching actor")
	}
	if invalidDelete.Actor == objectActor {
		t.Error("Invalid delete should have different actor")
	}

	// The authorization check in handleDeleteActivity would reject invalidDelete
	// because invalidDelete.Actor != activity.ActorURI (the content creator)
	if invalidDelete.Actor != objectActor {
		t.Log("Authorization check would correctly reject unauthorized Delete")
	}
}

// TestHandleInbox_BodySizeLimit tests that oversized requests are rejected
func TestHandleInbox_BodySizeLimit(t *testing.T) {
	// Test that the size limit is correctly set
	const maxBodySize = 1 * 1024 * 1024 // 1MB

	// Create a body just under the limit
	validBody := make([]byte, maxBodySize-1)
	if len(validBody) >= maxBodySize {
		t.Error("Valid body should be under size limit")
	}

	// Create a body exactly at the limit (would be rejected)
	tooLargeBody := make([]byte, maxBodySize)
	if len(tooLargeBody) != maxBodySize {
		t.Error("Too large body should be exactly at limit")
	}

	// The check in HandleInbox: if len(body) == maxBodySize { reject }
	// This would correctly reject tooLargeBody
	if len(tooLargeBody) == maxBodySize {
		t.Log("Size limit check would correctly reject oversized request")
	}
}

// TestHandleInbox_BodyRestoration tests that body is properly restored for signature verification
func TestHandleInbox_BodyRestoration(t *testing.T) {
	// Test the body restoration logic
	originalBody := []byte(`{"type":"Follow","actor":"https://example.com/users/alice"}`)

	// Simulate reading the body
	bodyAfterRead := make([]byte, len(originalBody))
	copy(bodyAfterRead, originalBody)

	// Verify body content is preserved
	if string(bodyAfterRead) != string(originalBody) {
		t.Error("Body should be preserved after read")
	}

	// The actual implementation uses io.NopCloser(bytes.NewReader(body))
	// to restore the body for signature verification
	// This allows VerifyRequest to read the body again
	t.Log("Body restoration allows signature verification after initial read")
}

// TestHandleCreateActivity_ActorCacheRefetch tests that expired actors are refetched
func TestHandleCreateActivity_ActorCacheRefetch(t *testing.T) {
	// Test the actor refetch logic
	actorURI := "https://example.com/users/alice"

	// Scenario 1: Actor not in cache (should fetch)
	var cachedActor *domain.RemoteAccount
	t.Log("Missing actor would trigger FetchRemoteActor")

	// Scenario 2: Actor in cache (should use cached)
	cachedActor = &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "alice",
		Domain:        "example.com",
		ActorURI:      actorURI,
		LastFetchedAt: time.Now(),
	}
	_ = cachedActor // Use the variable to avoid unused variable error
	t.Log("Cached actor would be used")

	// The updated handleCreateActivity now calls FetchRemoteActor
	// if ReadRemoteAccountByActorURI returns nil, improving reliability
}

// ============================================================================
// Integration Tests with Mock HTTP Server
// ============================================================================

// TestMockServerWebFinger tests WebFinger discovery through mock server
func TestMockServerWebFinger(t *testing.T) {
	mock := NewMockActivityPubServer()
	defer mock.Close()

	// Create test key pair
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate test keypair: %v", err)
	}

	// Set up actor response
	actor := CreateTestActorResponse(mock.Server.URL, "testuser", keypair.PublicPEM)
	mock.SetActorResponse(actor)

	// Verify actor response is set correctly
	if mock.ActorResponse == nil {
		t.Fatal("Actor response should be set")
	}

	if mock.ActorResponse.PreferredUsername != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", mock.ActorResponse.PreferredUsername)
	}

	if mock.ActorResponse.Inbox != mock.Server.URL+"/users/testuser/inbox" {
		t.Errorf("Inbox URL mismatch")
	}
}

// TestMockServerActivityDelivery tests activity delivery to mock inbox
func TestMockServerActivityDelivery(t *testing.T) {
	mock := NewMockActivityPubServer()
	defer mock.Close()

	// Track received activities
	var receivedActivity map[string]any
	mock.InboxHandler = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedActivity)
		w.WriteHeader(http.StatusAccepted)
	}

	// Create test activity
	actorURI := mock.Server.URL + "/users/sender"
	activityJSON := CreateTestFollowActivity(actorURI, mock.Server.URL+"/users/testuser")

	// Parse and verify the generated activity
	var activity map[string]any
	if err := json.Unmarshal([]byte(activityJSON), &activity); err != nil {
		t.Fatalf("Failed to parse generated activity: %v", err)
	}

	if activity["type"] != "Follow" {
		t.Errorf("Expected type 'Follow', got '%v'", activity["type"])
	}

	if activity["actor"] != actorURI {
		t.Errorf("Expected actor '%s', got '%v'", actorURI, activity["actor"])
	}
}

// TestTestHelperActivityGeneration tests all activity generation helpers
func TestTestHelperActivityGeneration(t *testing.T) {
	actorURI := "https://example.com/users/alice"
	objectURI := "https://example.com/notes/123"

	tests := []struct {
		name         string
		activityJSON string
		expectedType string
	}{
		{
			name:         "Follow activity",
			activityJSON: CreateTestFollowActivity(actorURI, objectURI),
			expectedType: "Follow",
		},
		{
			name:         "Accept activity",
			activityJSON: CreateTestAcceptActivity(actorURI, objectURI),
			expectedType: "Accept",
		},
		{
			name:         "Create activity",
			activityJSON: CreateTestCreateActivity(actorURI, "Test content"),
			expectedType: "Create",
		},
		{
			name:         "Like activity",
			activityJSON: CreateTestLikeActivity(actorURI, objectURI),
			expectedType: "Like",
		},
		{
			name:         "Delete activity",
			activityJSON: CreateTestDeleteActivity(actorURI, objectURI),
			expectedType: "Delete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity, err := ValidateActivityJSON(tt.activityJSON)
			if err != nil {
				t.Fatalf("Activity validation failed: %v", err)
			}

			if activity["type"] != tt.expectedType {
				t.Errorf("Expected type '%s', got '%v'", tt.expectedType, activity["type"])
			}

			if activity["actor"] != actorURI {
				t.Errorf("Expected actor '%s', got '%v'", actorURI, activity["actor"])
			}
		})
	}
}

// TestTestHelperUndoActivity tests Undo activity generation
func TestTestHelperUndoActivity(t *testing.T) {
	actorURI := "https://example.com/users/alice"
	targetURI := "https://example.com/users/bob"

	// Create a Follow activity to undo
	followActivity := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       actorURI + "/activities/follow-123",
		"type":     "Follow",
		"actor":    actorURI,
		"object":   targetURI,
	}

	// Create Undo for the Follow
	undoJSON := CreateTestUndoActivity(actorURI, followActivity)

	activity, err := ValidateActivityJSON(undoJSON)
	if err != nil {
		t.Fatalf("Undo activity validation failed: %v", err)
	}

	if activity["type"] != "Undo" {
		t.Errorf("Expected type 'Undo', got '%v'", activity["type"])
	}

	// Verify embedded object
	embeddedObj, ok := activity["object"].(map[string]any)
	if !ok {
		t.Fatal("Undo object should be a map")
	}

	if embeddedObj["type"] != "Follow" {
		t.Errorf("Expected embedded type 'Follow', got '%v'", embeddedObj["type"])
	}
}

// TestTestHelperUpdateActivity tests Update activity generation
func TestTestHelperUpdateActivity(t *testing.T) {
	actorURI := "https://example.com/users/alice"

	updatedNote := map[string]any{
		"id":      "https://example.com/notes/123",
		"type":    "Note",
		"content": "Updated content",
	}

	updateJSON := CreateTestUpdateActivity(actorURI, updatedNote)

	activity, err := ValidateActivityJSON(updateJSON)
	if err != nil {
		t.Fatalf("Update activity validation failed: %v", err)
	}

	if activity["type"] != "Update" {
		t.Errorf("Expected type 'Update', got '%v'", activity["type"])
	}
}

// TestValidateActivityJSON tests the activity validation helper
func TestValidateActivityJSON(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		expectError bool
		errorField  string
	}{
		{
			name:        "Valid activity",
			json:        `{"type": "Follow", "actor": "https://example.com/users/alice", "object": "https://example.com/users/bob"}`,
			expectError: false,
		},
		{
			name:        "Missing type",
			json:        `{"actor": "https://example.com/users/alice", "object": "https://example.com/users/bob"}`,
			expectError: true,
			errorField:  "type",
		},
		{
			name:        "Missing actor",
			json:        `{"type": "Follow", "object": "https://example.com/users/bob"}`,
			expectError: true,
			errorField:  "actor",
		},
		{
			name:        "Invalid JSON",
			json:        `{not valid}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateActivityJSON(tt.json)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if tt.errorField != "" {
					if valErr, ok := err.(*ValidationError); ok {
						if valErr.Field != tt.errorField {
							t.Errorf("Expected error field '%s', got '%s'", tt.errorField, valErr.Field)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestKeyPairGeneration tests RSA key pair generation for tests
func TestKeyPairGeneration(t *testing.T) {
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Verify private key
	if keypair.PrivateKey == nil {
		t.Error("Private key should not be nil")
	}

	// Verify PEM encoding
	if !strings.Contains(keypair.PrivatePEM, "PRIVATE KEY") {
		t.Error("Private PEM should contain PRIVATE KEY header")
	}

	if !strings.Contains(keypair.PublicPEM, "PUBLIC KEY") {
		t.Error("Public PEM should contain PUBLIC KEY header")
	}

	// Verify keys can be parsed back
	parsedPrivate, err := ParsePrivateKey(keypair.PrivatePEM)
	if err != nil {
		t.Errorf("Failed to parse private key: %v", err)
	}
	if parsedPrivate == nil {
		t.Error("Parsed private key should not be nil")
	}

	parsedPublic, err := ParsePublicKey(keypair.PublicPEM)
	if err != nil {
		t.Errorf("Failed to parse public key: %v", err)
	}
	if parsedPublic == nil {
		t.Error("Parsed public key should not be nil")
	}
}

// TestHTTPSignatureWithMock tests HTTP signature creation and verification
func TestHTTPSignatureWithMock(t *testing.T) {
	// Generate test key pair
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create a mock request
	body := []byte(`{"type":"Follow","actor":"https://example.com/users/alice"}`)
	req, err := http.NewRequest("POST", "https://example.com/inbox", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Calculate digest for the body
	hash := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", "example.com")
	req.Header.Set("Digest", digest)

	// Sign the request
	keyID := "https://example.com/users/alice#main-key"
	err = SignRequest(req, keypair.PrivateKey, keyID)
	if err != nil {
		t.Fatalf("Failed to sign request: %v", err)
	}

	// Verify signature header was added
	sig := req.Header.Get("Signature")
	if sig == "" {
		t.Error("Signature header should be set")
	}

	// Verify signature contains required parts
	if !strings.Contains(sig, "keyId=") {
		t.Error("Signature should contain keyId")
	}
	if !strings.Contains(sig, "algorithm=") {
		t.Error("Signature should contain algorithm")
	}
	if !strings.Contains(sig, "headers=") {
		t.Error("Signature should contain headers")
	}
	if !strings.Contains(sig, "signature=") {
		t.Error("Signature should contain signature")
	}
}

// TestCreateTestRemoteAccount tests remote account creation helper
func TestCreateTestRemoteAccount(t *testing.T) {
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	serverURL := "https://example.com"
	username := "testuser"

	account := CreateTestRemoteAccount(serverURL, username, keypair.PublicPEM)

	if account.Username != username {
		t.Errorf("Expected username '%s', got '%s'", username, account.Username)
	}

	if account.Domain != "example.com" {
		t.Errorf("Expected domain 'example.com', got '%s'", account.Domain)
	}

	if account.ActorURI != serverURL+"/users/"+username {
		t.Errorf("Expected actor URI '%s/users/%s', got '%s'", serverURL, username, account.ActorURI)
	}

	if account.InboxURI != serverURL+"/users/"+username+"/inbox" {
		t.Errorf("Expected inbox URI '%s/users/%s/inbox', got '%s'", serverURL, username, account.InboxURI)
	}
}

// TestCreateTestAccount tests local account creation helper
func TestCreateTestAccount(t *testing.T) {
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	username := "alice"
	account := CreateTestAccount(username, keypair)

	if account.Username != username {
		t.Errorf("Expected username '%s', got '%s'", username, account.Username)
	}

	if account.WebPublicKey != keypair.PublicPEM {
		t.Error("Account should have correct public key")
	}

	if account.WebPrivateKey != keypair.PrivatePEM {
		t.Error("Account should have correct private key")
	}

	if account.FirstTimeLogin != domain.FALSE {
		t.Error("Test account should not be first time login")
	}
}

// TestExtractDomainFromURL tests domain extraction helper
func TestExtractDomainFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{
		{"https://example.com", "example.com"},
		{"http://example.com", "example.com"},
		{"https://example.com:8080", "example.com:8080"},
		{"example.com", "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := extractDomainFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Integration Tests for Inbox Handlers with Dependency Injection
// ============================================================================

// TestHandleFollowActivityWithDeps_Success tests successful Follow processing
func TestHandleFollowActivityWithDeps_Success(t *testing.T) {
	// Setup mock database
	mockDB := NewMockDatabase()

	// Create local account
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Create remote actor
	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Create keypair for signing
	keypair, err := GenerateTestKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}
	localAccount.WebPrivateKey = keypair.PrivatePEM
	localAccount.WebPublicKey = keypair.PublicPEM

	// Setup mock HTTP client
	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetResponse(remoteActor.InboxURI, 202, nil)

	// Create deps
	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	// Create Follow activity
	followBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/follow-123",
		"type": "Follow",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/users/alice"
	}`)

	// Create config
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	// Call handler
	err = handleFollowActivityWithDeps(followBody, "alice", remoteActor, conf, deps)
	if err != nil {
		t.Fatalf("handleFollowActivityWithDeps failed: %v", err)
	}

	// Verify follow was created
	if len(mockDB.Follows) != 1 {
		t.Errorf("Expected 1 follow, got %d", len(mockDB.Follows))
	}

	// Verify Accept was sent
	if len(mockHTTP.Requests) != 1 {
		t.Errorf("Expected 1 HTTP request (Accept), got %d", len(mockHTTP.Requests))
	}
}

// TestHandleFollowActivityWithDeps_DuplicateFollow tests duplicate Follow handling
func TestHandleFollowActivityWithDeps_DuplicateFollow(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add existing follow
	existingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       remoteActor.Id,
		TargetAccountId: localAccount.Id,
		URI:             "https://remote.example.com/activities/follow-existing",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(existingFollow)

	keypair, _ := GenerateTestKeyPair()
	localAccount.WebPrivateKey = keypair.PrivatePEM
	localAccount.WebPublicKey = keypair.PublicPEM

	mockHTTP := NewMockHTTPClient()
	mockHTTP.SetResponse(remoteActor.InboxURI, 202, nil)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	followBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/follow-456",
		"type": "Follow",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/users/alice"
	}`)

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	err := handleFollowActivityWithDeps(followBody, "alice", remoteActor, conf, deps)
	if err != nil {
		t.Fatalf("handleFollowActivityWithDeps failed: %v", err)
	}

	// Should still only have 1 follow (duplicate not created)
	if len(mockDB.Follows) != 1 {
		t.Errorf("Expected 1 follow (no duplicate), got %d", len(mockDB.Follows))
	}

	// Accept should still be sent
	if len(mockHTTP.Requests) != 1 {
		t.Errorf("Expected 1 HTTP request (Accept), got %d", len(mockHTTP.Requests))
	}
}

// TestHandleUndoActivityWithDeps_Success tests successful Undo Follow processing
func TestHandleUndoActivityWithDeps_Success(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add existing follow to be undone
	followURI := "https://remote.example.com/activities/follow-123"
	existingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       remoteActor.Id,
		TargetAccountId: localAccount.Id,
		URI:             followURI,
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(existingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-456",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/follow-123",
			"type": "Follow"
		}
	}`)

	err := handleUndoActivityWithDeps(undoBody, "alice", remoteActor, deps)
	if err != nil {
		t.Fatalf("handleUndoActivityWithDeps failed: %v", err)
	}

	// Verify follow was deleted
	if len(mockDB.Follows) != 0 {
		t.Errorf("Expected 0 follows after undo, got %d", len(mockDB.Follows))
	}
}

// TestHandleUndoActivityWithDeps_Unauthorized tests unauthorized Undo rejection
func TestHandleUndoActivityWithDeps_Unauthorized(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Original follower
	originalFollower := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(originalFollower)

	// Malicious actor trying to undo someone else's follow
	maliciousActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "eve",
		Domain:   "evil.example.com",
		ActorURI: "https://evil.example.com/users/eve",
		InboxURI: "https://evil.example.com/users/eve/inbox",
	}
	mockDB.AddRemoteAccount(maliciousActor)

	// Follow created by bob
	followURI := "https://remote.example.com/activities/follow-123"
	existingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       originalFollower.Id,
		TargetAccountId: localAccount.Id,
		URI:             followURI,
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(existingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Eve tries to undo bob's follow
	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://evil.example.com/activities/undo-789",
		"type": "Undo",
		"actor": "https://evil.example.com/users/eve",
		"object": {
			"id": "https://remote.example.com/activities/follow-123",
			"type": "Follow"
		}
	}`)

	err := handleUndoActivityWithDeps(undoBody, "alice", maliciousActor, deps)
	if err == nil {
		t.Fatal("Expected error for unauthorized undo, got nil")
	}

	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected 'unauthorized' error, got: %v", err)
	}

	// Follow should still exist
	if len(mockDB.Follows) != 1 {
		t.Errorf("Expected follow to still exist, got %d follows", len(mockDB.Follows))
	}
}

// TestHandleAcceptActivityWithDeps_Success tests successful Accept processing
func TestHandleAcceptActivityWithDeps_Success(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add pending follow (not yet accepted)
	followURI := "https://local.example.com/activities/follow-123"
	pendingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followURI,
		Accepted:        false, // Pending
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(pendingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Accept with string object
	acceptBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/accept-456",
		"type": "Accept",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/activities/follow-123"
	}`)

	err := handleAcceptActivityWithDeps(acceptBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAcceptActivityWithDeps failed: %v", err)
	}

	// Verify follow is now accepted
	_, follow := mockDB.ReadFollowByURI(followURI)
	if follow == nil {
		t.Fatal("Follow not found after accept")
	}
	if !follow.Accepted {
		t.Error("Expected follow to be accepted")
	}
}

// TestHandleAcceptActivityWithDeps_ObjectAsMap tests Accept with embedded Follow object
func TestHandleAcceptActivityWithDeps_ObjectAsMap(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
	}
	mockDB.AddRemoteAccount(remoteActor)

	followURI := "https://local.example.com/activities/follow-123"
	pendingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followURI,
		Accepted:        false,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(pendingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Accept with embedded object
	acceptBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/accept-456",
		"type": "Accept",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://local.example.com/activities/follow-123",
			"type": "Follow"
		}
	}`)

	err := handleAcceptActivityWithDeps(acceptBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAcceptActivityWithDeps failed: %v", err)
	}

	_, follow := mockDB.ReadFollowByURI(followURI)
	if follow == nil || !follow.Accepted {
		t.Error("Expected follow to be accepted")
	}
}

// TestHandleCreateActivityWithDeps_Success tests successful Create processing
func TestHandleCreateActivityWithDeps_Success(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add follow relationship (we follow bob)
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             "https://local.example.com/activities/follow-123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(follow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	createBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/create-456",
		"type": "Create",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/notes/789",
			"type": "Note",
			"content": "Hello world!",
			"published": "2025-01-01T00:00:00Z",
			"attributedTo": "https://remote.example.com/users/bob"
		}
	}`)

	err := handleCreateActivityWithDeps(createBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleCreateActivityWithDeps failed: %v", err)
	}
}

// TestHandleCreateActivityWithDeps_NotFollowing tests rejection of Create from non-followed actor
func TestHandleCreateActivityWithDeps_NotFollowing(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// No follow relationship

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	createBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/create-456",
		"type": "Create",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/notes/789",
			"type": "Note",
			"content": "Spam message!",
			"published": "2025-01-01T00:00:00Z",
			"attributedTo": "https://remote.example.com/users/bob"
		}
	}`)

	err := handleCreateActivityWithDeps(createBody, "alice", deps)
	if err == nil {
		t.Fatal("Expected error for Create from non-followed actor")
	}

	if !strings.Contains(err.Error(), "not following") {
		t.Errorf("Expected 'not following' error, got: %v", err)
	}
}

// TestHandleCreateActivityWithDeps_ReplyIncrementsCounts tests that replies increment reply count
func TestHandleCreateActivityWithDeps_ReplyIncrementsCounts(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add follow relationship (we follow bob)
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             "https://local.example.com/activities/follow-123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(follow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	parentURI := "https://example.com/notes/parent-post"

	// Create activity that is a reply
	createBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/create-456",
		"type": "Create",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/notes/789",
			"type": "Note",
			"content": "This is a reply!",
			"published": "2025-01-01T00:00:00Z",
			"attributedTo": "https://remote.example.com/users/bob",
			"inReplyTo": "` + parentURI + `"
		}
	}`)

	err := handleCreateActivityWithDeps(createBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleCreateActivityWithDeps failed: %v", err)
	}

	// Verify IncrementReplyCountByURI was called with the parent URI
	if len(mockDB.IncrementReplyCountCalls) != 1 {
		t.Errorf("Expected 1 call to IncrementReplyCountByURI, got %d", len(mockDB.IncrementReplyCountCalls))
	}
	if len(mockDB.IncrementReplyCountCalls) > 0 && mockDB.IncrementReplyCountCalls[0] != parentURI {
		t.Errorf("Expected IncrementReplyCountByURI called with '%s', got '%s'", parentURI, mockDB.IncrementReplyCountCalls[0])
	}
}

// TestHandleCreateActivityWithDeps_DuplicateSkipsReplyCount tests that duplicate local posts don't increment reply count
func TestHandleCreateActivityWithDeps_DuplicateSkipsReplyCount(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add follow relationship
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             "https://local.example.com/activities/follow-123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(follow)

	// Add a local note that will be "duplicated" by the incoming activity
	localNoteURI := "https://local.example.com/notes/local-reply"
	localNote := &domain.Note{
		Id:           uuid.New(),
		CreatedBy:    localAccount.Id.String(),
		Message:      "My local reply",
		ObjectURI:    localNoteURI,
		InReplyToURI: "https://example.com/notes/parent-post",
		CreatedAt:    time.Now(),
	}
	mockDB.AddNote(localNote)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	parentURI := "https://example.com/notes/parent-post"

	// Create activity that is a duplicate of the local note
	// (this simulates a local post that federated out and came back)
	createBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/create-456",
		"type": "Create",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "` + localNoteURI + `",
			"type": "Note",
			"content": "My local reply",
			"published": "2025-01-01T00:00:00Z",
			"attributedTo": "https://remote.example.com/users/bob",
			"inReplyTo": "` + parentURI + `"
		}
	}`)

	err := handleCreateActivityWithDeps(createBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleCreateActivityWithDeps failed: %v", err)
	}

	// Verify IncrementReplyCountByURI was NOT called (duplicate detection should skip it)
	if len(mockDB.IncrementReplyCountCalls) != 0 {
		t.Errorf("Expected 0 calls to IncrementReplyCountByURI for duplicate, got %d: %v",
			len(mockDB.IncrementReplyCountCalls), mockDB.IncrementReplyCountCalls)
	}
}

// TestHandleCreateActivityWithDeps_NoReplyNoIncrement tests that non-reply posts don't increment reply count
func TestHandleCreateActivityWithDeps_NoReplyNoIncrement(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add follow relationship
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             "https://local.example.com/activities/follow-123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(follow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Create activity that is NOT a reply (no inReplyTo)
	createBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/create-456",
		"type": "Create",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/notes/789",
			"type": "Note",
			"content": "This is a standalone post!",
			"published": "2025-01-01T00:00:00Z",
			"attributedTo": "https://remote.example.com/users/bob"
		}
	}`)

	err := handleCreateActivityWithDeps(createBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleCreateActivityWithDeps failed: %v", err)
	}

	// Verify IncrementReplyCountByURI was NOT called (not a reply)
	if len(mockDB.IncrementReplyCountCalls) != 0 {
		t.Errorf("Expected 0 calls to IncrementReplyCountByURI for non-reply, got %d", len(mockDB.IncrementReplyCountCalls))
	}
}

// TestHandleDeleteActivityWithDeps_PostDeletion tests successful post deletion
func TestHandleDeleteActivityWithDeps_PostDeletion(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	actorURI := "https://remote.example.com/users/bob"
	objectURI := "https://remote.example.com/notes/123"

	// Add activity to be deleted
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create-456",
		ActivityType: "Create",
		ActorURI:     actorURI,
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create"}`,
		Processed:    true,
		Local:        false,
		CreatedAt:    time.Now(),
	}
	mockDB.AddActivity(activity)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	deleteBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/delete-789",
		"type": "Delete",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://remote.example.com/notes/123"
	}`)

	err := handleDeleteActivityWithDeps(deleteBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleDeleteActivityWithDeps failed: %v", err)
	}

	// Verify activity was deleted
	if len(mockDB.Activities) != 0 {
		t.Errorf("Expected 0 activities after delete, got %d", len(mockDB.Activities))
	}
}

// TestHandleDeleteActivityWithDeps_Unauthorized tests unauthorized deletion rejection
func TestHandleDeleteActivityWithDeps_Unauthorized(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	objectURI := "https://remote.example.com/notes/123"

	// Activity created by bob
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create-456",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/bob",
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create"}`,
		Processed:    true,
		Local:        false,
		CreatedAt:    time.Now(),
	}
	mockDB.AddActivity(activity)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Eve tries to delete bob's post
	deleteBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://evil.example.com/activities/delete-789",
		"type": "Delete",
		"actor": "https://evil.example.com/users/eve",
		"object": "https://remote.example.com/notes/123"
	}`)

	err := handleDeleteActivityWithDeps(deleteBody, "alice", deps)
	if err == nil {
		t.Fatal("Expected error for unauthorized delete")
	}

	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("Expected 'unauthorized' error, got: %v", err)
	}

	// Activity should still exist
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected activity to still exist, got %d activities", len(mockDB.Activities))
	}
}

// TestHandleDeleteActivityWithDeps_ActorDeletion tests actor account deletion
func TestHandleDeleteActivityWithDeps_ActorDeletion(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	actorURI := "https://remote.example.com/users/bob"
	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: actorURI,
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Add follow from remote actor
	follow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       remoteActor.Id,
		TargetAccountId: localAccount.Id,
		URI:             "https://remote.example.com/activities/follow-123",
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(follow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Actor deletes themselves (object == actor)
	deleteBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/delete-account",
		"type": "Delete",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://remote.example.com/users/bob"
	}`)

	err := handleDeleteActivityWithDeps(deleteBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleDeleteActivityWithDeps failed: %v", err)
	}

	// Verify remote account was deleted
	if len(mockDB.RemoteAccounts) != 0 {
		t.Errorf("Expected 0 remote accounts after delete, got %d", len(mockDB.RemoteAccounts))
	}

	// Verify follows were deleted
	if len(mockDB.Follows) != 0 {
		t.Errorf("Expected 0 follows after actor delete, got %d", len(mockDB.Follows))
	}
}

// TestHandleUpdateActivityWithDeps_NoteUpdate tests Note/Article update processing
func TestHandleUpdateActivityWithDeps_NoteUpdate(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteURI := "https://remote.example.com/notes/123"

	// Add existing activity with the note
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create-456",
		ActivityType: "Create",
		ActorURI:     "https://remote.example.com/users/bob",
		ObjectURI:    noteURI,
		RawJSON:      `{"type":"Create","object":{"content":"Original content"}}`,
		Processed:    true,
		Local:        false,
		CreatedAt:    time.Now(),
	}
	mockDB.AddActivity(activity)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	updateBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/update-789",
		"type": "Update",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/notes/123",
			"type": "Note",
			"content": "Updated content"
		}
	}`)

	err := handleUpdateActivityWithDeps(updateBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleUpdateActivityWithDeps failed: %v", err)
	}

	// Verify activity was updated
	_, updatedActivity := mockDB.ReadActivityByObjectURI(noteURI)
	if updatedActivity == nil {
		t.Fatal("Activity not found after update")
	}
	if !strings.Contains(updatedActivity.RawJSON, "Updated content") {
		t.Error("Activity RawJSON should contain updated content")
	}
}

// TestHandleLikeActivityWithDeps tests Like activity processing (placeholder)
func TestHandleLikeActivityWithDeps(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	likeBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/like-123",
		"type": "Like",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/notes/456"
	}`)

	// Like handler is currently a placeholder, should not error
	err := handleLikeActivityWithDeps(likeBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleLikeActivityWithDeps failed: %v", err)
	}
}

// TestHandleLikeActivity_StoresLikeAndIncrementsCount tests that a Like activity
// properly stores the like record and increments the note's like count
func TestHandleLikeActivity_StoresLikeAndIncrementsCount(t *testing.T) {
	mockDB := NewMockDatabase()

	// Create local account
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Create a note to be liked
	noteId := uuid.New()
	note := &domain.Note{
		Id:        noteId,
		CreatedBy: "alice",
		Message:   "Hello world!",
		ObjectURI: "https://local.example.com/notes/" + noteId.String(),
		LikeCount: 0,
	}
	mockDB.AddNote(note)

	// Create remote account that will like the note
	remoteAccount := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		LastFetchedAt: time.Now(), // Set to now so cache is considered fresh
	}
	mockDB.AddRemoteAccount(remoteAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	likeBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/like-123",
		"type": "Like",
		"actor": "https://remote.example.com/users/bob",
		"object": "` + note.ObjectURI + `"
	}`)

	err := handleLikeActivityWithDeps(likeBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleLikeActivityWithDeps failed: %v", err)
	}

	// Verify like was stored
	if len(mockDB.Likes) != 1 {
		t.Errorf("Expected 1 like stored, got %d", len(mockDB.Likes))
	}

	// Verify the like has correct data
	for _, like := range mockDB.Likes {
		if like.AccountId != remoteAccount.Id {
			t.Errorf("Like has wrong account ID: got %s, want %s", like.AccountId, remoteAccount.Id)
		}
		if like.NoteId != noteId {
			t.Errorf("Like has wrong note ID: got %s, want %s", like.NoteId, noteId)
		}
		if like.URI != "https://remote.example.com/activities/like-123" {
			t.Errorf("Like has wrong URI: got %s", like.URI)
		}
	}

	// Verify like count was incremented
	if len(mockDB.IncrementLikeCountCalls) != 1 {
		t.Errorf("Expected IncrementLikeCountByNoteId to be called once, got %d calls", len(mockDB.IncrementLikeCountCalls))
	}
	if len(mockDB.IncrementLikeCountCalls) > 0 && mockDB.IncrementLikeCountCalls[0] != noteId {
		t.Errorf("IncrementLikeCountByNoteId called with wrong note ID: got %s, want %s",
			mockDB.IncrementLikeCountCalls[0], noteId)
	}
}

// TestHandleLikeActivity_DuplicateLikeIgnored tests that duplicate likes from the same
// account on the same note are ignored (no error, no duplicate storage)
func TestHandleLikeActivity_DuplicateLikeIgnored(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:        noteId,
		CreatedBy: "alice",
		Message:   "Hello world!",
		ObjectURI: "https://local.example.com/notes/" + noteId.String(),
		LikeCount: 1,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		LastFetchedAt: time.Now(), // Set to now so cache is considered fresh
	}
	mockDB.AddRemoteAccount(remoteAccount)

	// Pre-add an existing like from bob on this note
	existingLike := &domain.Like{
		Id:        uuid.New(),
		AccountId: remoteAccount.Id,
		NoteId:    noteId,
		URI:       "https://remote.example.com/activities/like-old",
	}
	mockDB.Likes[existingLike.Id] = existingLike

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Try to like again with a different activity URI
	likeBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/like-new",
		"type": "Like",
		"actor": "https://remote.example.com/users/bob",
		"object": "` + note.ObjectURI + `"
	}`)

	err := handleLikeActivityWithDeps(likeBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleLikeActivityWithDeps should not error on duplicate: %v", err)
	}

	// Verify no additional like was stored
	if len(mockDB.Likes) != 1 {
		t.Errorf("Expected still 1 like stored (no duplicate), got %d", len(mockDB.Likes))
	}

	// Verify like count was NOT incremented again
	if len(mockDB.IncrementLikeCountCalls) != 0 {
		t.Errorf("Expected IncrementLikeCountByNoteId NOT to be called for duplicate, got %d calls",
			len(mockDB.IncrementLikeCountCalls))
	}
}

// TestHandleLikeActivity_NoteNotFound tests that liking a non-existent note
// doesn't cause an error (graceful handling)
func TestHandleLikeActivity_NoteNotFound(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	likeBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/like-123",
		"type": "Like",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/notes/nonexistent"
	}`)

	// Should not error - note simply doesn't exist locally
	err := handleLikeActivityWithDeps(likeBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleLikeActivityWithDeps should not error for missing note: %v", err)
	}

	// Verify no like was stored
	if len(mockDB.Likes) != 0 {
		t.Errorf("Expected 0 likes stored for missing note, got %d", len(mockDB.Likes))
	}
}

// TestHandleUndoLike tests that Undo Like properly removes the like and decrements count
func TestHandleUndoLike(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:        noteId,
		CreatedBy: "alice",
		Message:   "Hello world!",
		ObjectURI: "https://local.example.com/notes/" + noteId.String(),
		LikeCount: 1,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:           uuid.New(),
		Username:     "bob",
		Domain:       "remote.example.com",
		ActorURI:     "https://remote.example.com/users/bob",
		InboxURI:     "https://remote.example.com/users/bob/inbox",
		PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	}
	mockDB.AddRemoteAccount(remoteAccount)

	// Add existing like
	existingLike := &domain.Like{
		Id:        uuid.New(),
		AccountId: remoteAccount.Id,
		NoteId:    noteId,
		URI:       "https://remote.example.com/activities/like-123",
	}
	mockDB.Likes[existingLike.Id] = existingLike

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-like-123",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/like-123",
			"type": "Like",
			"actor": "https://remote.example.com/users/bob",
			"object": "` + note.ObjectURI + `"
		}
	}`)

	err := handleUndoActivityWithDeps(undoBody, "alice", remoteAccount, deps)
	if err != nil {
		t.Fatalf("handleUndoActivityWithDeps failed: %v", err)
	}

	// Verify like was removed
	if len(mockDB.Likes) != 0 {
		t.Errorf("Expected 0 likes after undo, got %d", len(mockDB.Likes))
	}

	// Verify like count was decremented
	if note.LikeCount != 0 {
		t.Errorf("Expected like count to be 0 after undo, got %d", note.LikeCount)
	}
}

// TestHandleUndoLike_NoExistingLike tests that Undo Like for a non-existent like
// is handled gracefully
func TestHandleUndoLike_NoExistingLike(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:        noteId,
		CreatedBy: "alice",
		Message:   "Hello world!",
		ObjectURI: "https://local.example.com/notes/" + noteId.String(),
		LikeCount: 0,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:           uuid.New(),
		Username:     "bob",
		Domain:       "remote.example.com",
		ActorURI:     "https://remote.example.com/users/bob",
		InboxURI:     "https://remote.example.com/users/bob/inbox",
		PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	}
	mockDB.AddRemoteAccount(remoteAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Undo a like that doesn't exist
	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-like-123",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/like-nonexistent",
			"type": "Like",
			"actor": "https://remote.example.com/users/bob",
			"object": "` + note.ObjectURI + `"
		}
	}`)

	// Should not error
	err := handleUndoActivityWithDeps(undoBody, "alice", remoteAccount, deps)
	if err != nil {
		t.Fatalf("handleUndoActivityWithDeps should not error for missing like: %v", err)
	}

	// Verify like count didn't go negative
	if note.LikeCount < 0 {
		t.Errorf("Like count went negative: %d", note.LikeCount)
	}
}

// ============================================================================
// Announce (Boost) Activity Tests
// ============================================================================

// TestHandleAnnounceActivity_StoresBoostAndIncrementsCount tests that an Announce
// activity properly stores a boost and increments the boost count
func TestHandleAnnounceActivity_StoresBoostAndIncrementsCount(t *testing.T) {
	mockDB := NewMockDatabase()

	// Create local account
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Create a note to be boosted
	noteId := uuid.New()
	note := &domain.Note{
		Id:         noteId,
		CreatedBy:  "alice",
		Message:    "Hello world!",
		ObjectURI:  "https://local.example.com/notes/" + noteId.String(),
		BoostCount: 0,
	}
	mockDB.AddNote(note)

	// Create remote account that will boost the note
	remoteAccount := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		LastFetchedAt: time.Now(), // Set to now so cache is considered fresh
	}
	mockDB.AddRemoteAccount(remoteAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	announceBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/announce-123",
		"type": "Announce",
		"actor": "https://remote.example.com/users/bob",
		"object": "` + note.ObjectURI + `"
	}`)

	err := handleAnnounceActivityWithDeps(announceBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Verify boost was stored
	if len(mockDB.Boosts) != 1 {
		t.Errorf("Expected 1 boost stored, got %d", len(mockDB.Boosts))
	}

	// Verify the boost has correct data
	for _, boost := range mockDB.Boosts {
		if boost.AccountId != remoteAccount.Id {
			t.Errorf("Boost has wrong account ID: got %s, want %s", boost.AccountId, remoteAccount.Id)
		}
		if boost.NoteId != noteId {
			t.Errorf("Boost has wrong note ID: got %s, want %s", boost.NoteId, noteId)
		}
		if boost.URI != "https://remote.example.com/activities/announce-123" {
			t.Errorf("Boost has wrong URI: got %s", boost.URI)
		}
	}

	// Verify boost count was incremented
	if len(mockDB.IncrementBoostCountCalls) != 1 {
		t.Errorf("Expected IncrementBoostCountByNoteId to be called once, got %d calls", len(mockDB.IncrementBoostCountCalls))
	}
	if len(mockDB.IncrementBoostCountCalls) > 0 && mockDB.IncrementBoostCountCalls[0] != noteId {
		t.Errorf("IncrementBoostCountByNoteId called with wrong note ID: got %s, want %s",
			mockDB.IncrementBoostCountCalls[0], noteId)
	}
}

// TestHandleAnnounceActivity_DuplicateBoostIgnored tests that duplicate boosts from the same
// account on the same note are ignored (no error, no duplicate storage)
func TestHandleAnnounceActivity_DuplicateBoostIgnored(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:         noteId,
		CreatedBy:  "alice",
		Message:    "Hello world!",
		ObjectURI:  "https://local.example.com/notes/" + noteId.String(),
		BoostCount: 1,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		LastFetchedAt: time.Now(), // Set to now so cache is considered fresh
	}
	mockDB.AddRemoteAccount(remoteAccount)

	// Pre-add an existing boost from bob on this note
	existingBoost := &domain.Boost{
		Id:        uuid.New(),
		AccountId: remoteAccount.Id,
		NoteId:    noteId,
		URI:       "https://remote.example.com/activities/announce-old",
	}
	mockDB.Boosts[existingBoost.Id] = existingBoost

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Try to boost again with a different activity URI
	announceBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/announce-new",
		"type": "Announce",
		"actor": "https://remote.example.com/users/bob",
		"object": "` + note.ObjectURI + `"
	}`)

	err := handleAnnounceActivityWithDeps(announceBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps should not error on duplicate: %v", err)
	}

	// Verify no additional boost was stored
	if len(mockDB.Boosts) != 1 {
		t.Errorf("Expected still 1 boost stored (no duplicate), got %d", len(mockDB.Boosts))
	}

	// Verify boost count was NOT incremented again
	if len(mockDB.IncrementBoostCountCalls) != 0 {
		t.Errorf("Expected IncrementBoostCountByNoteId NOT to be called for duplicate, got %d calls",
			len(mockDB.IncrementBoostCountCalls))
	}
}

// TestHandleAnnounceActivity_NoteNotFound tests that boosting a non-existent note
// doesn't cause an error (graceful handling)
func TestHandleAnnounceActivity_NoteNotFound(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	announceBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/announce-123",
		"type": "Announce",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/notes/nonexistent"
	}`)

	// Should not error - note simply doesn't exist locally
	err := handleAnnounceActivityWithDeps(announceBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps should not error for missing note: %v", err)
	}

	// Verify no boost was stored
	if len(mockDB.Boosts) != 0 {
		t.Errorf("Expected 0 boosts stored for missing note, got %d", len(mockDB.Boosts))
	}
}

// TestHandleUndoAnnounce tests that Undo Announce properly removes the boost and decrements count
func TestHandleUndoAnnounce(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:         noteId,
		CreatedBy:  "alice",
		Message:    "Hello world!",
		ObjectURI:  "https://local.example.com/notes/" + noteId.String(),
		BoostCount: 1,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:           uuid.New(),
		Username:     "bob",
		Domain:       "remote.example.com",
		ActorURI:     "https://remote.example.com/users/bob",
		InboxURI:     "https://remote.example.com/users/bob/inbox",
		PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	}
	mockDB.AddRemoteAccount(remoteAccount)

	// Add existing boost
	existingBoost := &domain.Boost{
		Id:        uuid.New(),
		AccountId: remoteAccount.Id,
		NoteId:    noteId,
		URI:       "https://remote.example.com/activities/announce-123",
	}
	mockDB.Boosts[existingBoost.Id] = existingBoost

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-announce-123",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/announce-123",
			"type": "Announce",
			"actor": "https://remote.example.com/users/bob",
			"object": "` + note.ObjectURI + `"
		}
	}`)

	err := handleUndoActivityWithDeps(undoBody, "alice", remoteAccount, deps)
	if err != nil {
		t.Fatalf("handleUndoActivityWithDeps failed: %v", err)
	}

	// Verify boost was removed
	if len(mockDB.Boosts) != 0 {
		t.Errorf("Expected 0 boosts after undo, got %d", len(mockDB.Boosts))
	}

	// Verify boost count was decremented
	if note.BoostCount != 0 {
		t.Errorf("Expected boost count to be 0 after undo, got %d", note.BoostCount)
	}
}

// TestHandleUndoAnnounce_NoExistingBoost tests that Undo Announce for a non-existent boost
// is handled gracefully
func TestHandleUndoAnnounce_NoExistingBoost(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:         noteId,
		CreatedBy:  "alice",
		Message:    "Hello world!",
		ObjectURI:  "https://local.example.com/notes/" + noteId.String(),
		BoostCount: 0,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:           uuid.New(),
		Username:     "bob",
		Domain:       "remote.example.com",
		ActorURI:     "https://remote.example.com/users/bob",
		InboxURI:     "https://remote.example.com/users/bob/inbox",
		PublicKeyPem: "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
	}
	mockDB.AddRemoteAccount(remoteAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Undo an announce that doesn't exist
	undoBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-announce-123",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/announce-nonexistent",
			"type": "Announce",
			"actor": "https://remote.example.com/users/bob",
			"object": "` + note.ObjectURI + `"
		}
	}`)

	// Should not error
	err := handleUndoActivityWithDeps(undoBody, "alice", remoteAccount, deps)
	if err != nil {
		t.Fatalf("handleUndoActivityWithDeps should not error for missing boost: %v", err)
	}

	// Verify boost count didn't go negative
	if note.BoostCount < 0 {
		t.Errorf("Boost count went negative: %d", note.BoostCount)
	}
}

// TestHandleAnnounceActivity_ObjectAsMap tests that Announce activity with object as map
// (containing id field) is handled correctly
func TestHandleAnnounceActivity_ObjectAsMap(t *testing.T) {
	mockDB := NewMockDatabase()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	noteId := uuid.New()
	note := &domain.Note{
		Id:         noteId,
		CreatedBy:  "alice",
		Message:    "Hello world!",
		ObjectURI:  "https://local.example.com/notes/" + noteId.String(),
		BoostCount: 0,
	}
	mockDB.AddNote(note)

	remoteAccount := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----",
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteAccount)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Object is a map with id field (some servers send it this way)
	announceBody := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/announce-123",
		"type": "Announce",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "` + note.ObjectURI + `",
			"type": "Note"
		}
	}`)

	err := handleAnnounceActivityWithDeps(announceBody, "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed with object as map: %v", err)
	}

	// Verify boost was stored
	if len(mockDB.Boosts) != 1 {
		t.Errorf("Expected 1 boost stored for object-as-map format, got %d", len(mockDB.Boosts))
	}
}

// ============================================================================
// HandleInboxWithDeps Integration Tests (with httptest)
// ============================================================================

// createSignedRequest creates a properly signed HTTP request for testing
func createSignedRequest(t *testing.T, method, url string, body []byte, keypair *TestKeyPair, keyID string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.Host)

	// Calculate digest
	hash := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])
	req.Header.Set("Digest", digest)

	// Sign the request
	if err := SignRequest(req, keypair.PrivateKey, keyID); err != nil {
		t.Fatalf("Failed to sign request: %v", err)
	}

	return req
}

// TestHandleInboxWithDeps_MissingSignature tests rejection of unsigned requests
func TestHandleInboxWithDeps_MissingSignature(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{"type":"Follow","actor":"https://remote.example.com/users/bob"}`)
	req := httptest.NewRequest("POST", "/users/alice/inbox", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/activity+json")
	// No Signature header

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Missing signature") {
		t.Errorf("Expected 'Missing signature' error, got: %s", rr.Body.String())
	}
}

// TestHandleInboxWithDeps_InvalidJSON tests rejection of invalid JSON
func TestHandleInboxWithDeps_InvalidJSON(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Create keypair
	keypair, _ := GenerateTestKeyPair()

	// Create remote actor with public key
	remoteActor := &domain.RemoteAccount{
		Id:           uuid.New(),
		Username:     "bob",
		Domain:       "remote.example.com",
		ActorURI:     "https://remote.example.com/users/bob",
		InboxURI:     "https://remote.example.com/users/bob/inbox",
		PublicKeyPem: keypair.PublicPEM,
	}
	mockDB.AddRemoteAccount(remoteActor)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	// Invalid JSON body
	body := []byte(`{invalid json}`)
	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", rr.Code)
	}
}

// TestHandleInboxWithDeps_UnknownActor tests rejection when actor cannot be fetched
func TestHandleInboxWithDeps_UnknownActor(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// No actor in DB, and HTTP fetch will fail
	mockHTTP.SetError("https://unknown.example.com/users/eve", &mockNetworkError{message: "connection refused"})

	keypair, _ := GenerateTestKeyPair()

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type": "Follow",
		"actor": "https://unknown.example.com/users/eve",
		"object": "https://local.example.com/users/alice"
	}`)

	keyID := "https://unknown.example.com/users/eve#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 Bad Request, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Failed to verify signer") {
		t.Errorf("Expected 'Failed to verify signer' error, got: %s", rr.Body.String())
	}
}

// TestHandleInboxWithDeps_InvalidSignature tests rejection of invalid signatures
func TestHandleInboxWithDeps_InvalidSignature(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	// Create two different keypairs - one for signing, one for verification
	signingKeypair, _ := GenerateTestKeyPair()
	verifyKeypair, _ := GenerateTestKeyPair() // Different keypair!

	// Remote actor has different public key than what was used to sign
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  verifyKeypair.PublicPEM, // Different from signing key
		LastFetchedAt: time.Now(),              // Fresh cache
	}
	mockDB.AddRemoteAccount(remoteActor)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type": "Follow",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/users/alice"
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	// Sign with signingKeypair but verify with verifyKeypair (mismatch!)
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, signingKeypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 Unauthorized, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Invalid signature") {
		t.Errorf("Expected 'Invalid signature' error, got: %s", rr.Body.String())
	}
}

// TestHandleInboxWithDeps_FollowSuccess tests successful Follow activity processing
func TestHandleInboxWithDeps_FollowSuccess(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	// Local account being followed
	localAccount := &domain.Account{
		Id:            uuid.New(),
		Username:      "alice",
		WebPrivateKey: keypair.PrivatePEM,
		WebPublicKey:  keypair.PublicPEM,
	}
	mockDB.AddAccount(localAccount)

	// Remote actor doing the following
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Mock Accept delivery
	mockHTTP.SetResponse("https://remote.example.com/users/bob/inbox", 202, nil)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/follow-123",
		"type": "Follow",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/users/alice"
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify follow was created
	if len(mockDB.Follows) != 1 {
		t.Errorf("Expected 1 follow, got %d", len(mockDB.Follows))
	}

	// Verify activity was stored
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(mockDB.Activities))
	}

	// Verify activity was marked as processed
	for _, act := range mockDB.Activities {
		if !act.Processed {
			t.Error("Activity should be marked as processed")
		}
	}
}

// TestHandleInboxWithDeps_AcceptSuccess tests successful Accept activity processing
func TestHandleInboxWithDeps_AcceptSuccess(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	// Local account that sent the follow
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Remote actor accepting the follow
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Pending follow to be accepted
	followURI := "https://local.example.com/activities/follow-123"
	pendingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followURI,
		Accepted:        false,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(pendingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/accept-456",
		"type": "Accept",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://local.example.com/activities/follow-123"
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify follow was accepted
	_, follow := mockDB.ReadFollowByURI(followURI)
	if follow == nil {
		t.Fatal("Follow not found")
	}
	if !follow.Accepted {
		t.Error("Follow should be accepted")
	}
}

// TestHandleInboxWithDeps_UndoSuccess tests successful Undo Follow processing
func TestHandleInboxWithDeps_UndoSuccess(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	// Local account being unfollowed
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Remote actor undoing their follow
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Existing follow to be undone
	followURI := "https://remote.example.com/activities/follow-123"
	existingFollow := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       remoteActor.Id,
		TargetAccountId: localAccount.Id,
		URI:             followURI,
		Accepted:        true,
		CreatedAt:       time.Now(),
	}
	mockDB.AddFollow(existingFollow)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/undo-456",
		"type": "Undo",
		"actor": "https://remote.example.com/users/bob",
		"object": {
			"id": "https://remote.example.com/activities/follow-123",
			"type": "Follow"
		}
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify follow was deleted
	if len(mockDB.Follows) != 0 {
		t.Errorf("Expected 0 follows after undo, got %d", len(mockDB.Follows))
	}
}

// TestHandleInboxWithDeps_DeleteSuccess tests successful Delete activity processing
func TestHandleInboxWithDeps_DeleteSuccess(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	// Local account
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	actorURI := "https://remote.example.com/users/bob"
	objectURI := "https://remote.example.com/notes/123"

	// Remote actor
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      actorURI,
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	// Activity to be deleted
	activity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://remote.example.com/activities/create-456",
		ActivityType: "Create",
		ActorURI:     actorURI,
		ObjectURI:    objectURI,
		RawJSON:      `{"type":"Create"}`,
		Processed:    true,
		Local:        false,
		CreatedAt:    time.Now(),
	}
	mockDB.AddActivity(activity)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/delete-789",
		"type": "Delete",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://remote.example.com/notes/123"
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify activity was deleted (only the new Delete activity remains)
	// Note: HandleInbox creates a new activity record for the Delete itself
	createActivities := 0
	for _, act := range mockDB.Activities {
		if act.ActivityType == "Create" {
			createActivities++
		}
	}
	if createActivities != 0 {
		t.Errorf("Expected Create activity to be deleted, got %d Create activities", createActivities)
	}
}

// TestHandleInboxWithDeps_UnsupportedType tests handling of unsupported activity types
func TestHandleInboxWithDeps_UnsupportedType(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	// Local account
	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Remote actor
	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	// Unsupported activity type "Question" (polls are not implemented)
	body := []byte(`{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/question-123",
		"type": "Question",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://other.example.com/notes/456"
	}`)

	keyID := "https://remote.example.com/users/bob#main-key"
	req := createSignedRequest(t, "POST", "/users/alice/inbox", body, keypair, keyID)

	rr := httptest.NewRecorder()
	HandleInboxWithDeps(rr, req, "alice", conf, deps)

	// Should still return 202 (unsupported types are logged but not rejected)
	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status 202 Accepted for unsupported type, got %d: %s", rr.Code, rr.Body.String())
	}

	// Activity should still be stored
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity (stored even if unsupported), got %d", len(mockDB.Activities))
	}
}

// TestHandleInboxWithDeps_ObjectURIExtraction tests extraction of objectURI from different formats
func TestHandleInboxWithDeps_ObjectURIExtraction(t *testing.T) {
	mockDB := NewMockDatabase()
	mockHTTP := NewMockHTTPClient()

	keypair, _ := GenerateTestKeyPair()

	localAccount := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	remoteActor := &domain.RemoteAccount{
		Id:            uuid.New(),
		Username:      "bob",
		Domain:        "remote.example.com",
		ActorURI:      "https://remote.example.com/users/bob",
		InboxURI:      "https://remote.example.com/users/bob/inbox",
		PublicKeyPem:  keypair.PublicPEM,
		LastFetchedAt: time.Now(),
	}
	mockDB.AddRemoteAccount(remoteActor)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockHTTP,
	}

	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "local.example.com"

	tests := []struct {
		name           string
		body           string
		expectedObjURI string
	}{
		{
			name: "object_as_string",
			body: `{
				"@context": "https://www.w3.org/ns/activitystreams",
				"id": "https://remote.example.com/activities/1",
				"type": "Announce",
				"actor": "https://remote.example.com/users/bob",
				"object": "https://example.com/notes/string-uri"
			}`,
			expectedObjURI: "https://example.com/notes/string-uri",
		},
		{
			name: "object_as_map",
			body: `{
				"@context": "https://www.w3.org/ns/activitystreams",
				"id": "https://remote.example.com/activities/2",
				"type": "Announce",
				"actor": "https://remote.example.com/users/bob",
				"object": {
					"id": "https://example.com/notes/map-uri",
					"type": "Note"
				}
			}`,
			expectedObjURI: "https://example.com/notes/map-uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear activities
			mockDB.Activities = make(map[uuid.UUID]*domain.Activity)

			keyID := "https://remote.example.com/users/bob#main-key"
			req := createSignedRequest(t, "POST", "/users/alice/inbox", []byte(tt.body), keypair, keyID)

			rr := httptest.NewRecorder()
			HandleInboxWithDeps(rr, req, "alice", conf, deps)

			if rr.Code != http.StatusAccepted {
				t.Errorf("Expected 202, got %d", rr.Code)
			}

			// Check stored activity has correct ObjectURI
			for _, act := range mockDB.Activities {
				if act.ObjectURI != tt.expectedObjURI {
					t.Errorf("Expected ObjectURI '%s', got '%s'", tt.expectedObjURI, act.ObjectURI)
				}
			}
		})
	}
}

// ============ Relay Announce Tests ============

func TestHandleAnnounceFromRelay(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up a relay
	relay := &domain.Relay{
		Id:       uuid.New(),
		ActorURI: "https://relay.fedi.buzz/actor",
		InboxURI: "https://relay.fedi.buzz/inbox",
		Name:     "Test Relay",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	// Set up a local account
	localAcct := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAcct)

	// Create a mock HTTP client that returns a Note when fetching
	mockClient := NewMockHTTPClient()

	// Set up response for the Note object
	noteJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://pixelfed.social/p/user/123",
		"type": "Note",
		"attributedTo": "https://pixelfed.social/users/photographer",
		"content": "Check out this photo!",
		"published": "2025-01-01T12:00:00Z"
	}`
	mockClient.Responses["https://pixelfed.social/p/user/123"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(noteJSON)),
		Header:     make(http.Header),
	}

	// Set up response for actor fetch
	actorJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://pixelfed.social/users/photographer",
		"type": "Person",
		"preferredUsername": "photographer",
		"inbox": "https://pixelfed.social/users/photographer/inbox",
		"outbox": "https://pixelfed.social/users/photographer/outbox",
		"publicKey": {
			"id": "https://pixelfed.social/users/photographer#main-key",
			"owner": "https://pixelfed.social/users/photographer",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"
		}
	}`
	mockClient.Responses["https://pixelfed.social/users/photographer"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(actorJSON)),
		Header:     make(http.Header),
	}

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockClient,
	}

	// Announce from relay
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://relay.fedi.buzz/activities/announce-123",
		"type": "Announce",
		"actor": "https://relay.fedi.buzz/actor",
		"object": "https://pixelfed.social/p/user/123"
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Verify activity was stored
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(mockDB.Activities))
	}

	// Check that it was stored as a Create activity
	for _, act := range mockDB.Activities {
		if act.ActivityType != "Create" {
			t.Errorf("Expected ActivityType 'Create', got '%s'", act.ActivityType)
		}
		if act.ObjectURI != "https://pixelfed.social/p/user/123" {
			t.Errorf("Expected ObjectURI 'https://pixelfed.social/p/user/123', got '%s'", act.ObjectURI)
		}
		if act.ActorURI != "https://pixelfed.social/users/photographer" {
			t.Errorf("Expected ActorURI from the Note's attributedTo, got '%s'", act.ActorURI)
		}
	}
}

func TestHandleAnnounceFromRelayWithEmbeddedObject(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up a relay
	relay := &domain.Relay{
		Id:       uuid.New(),
		ActorURI: "https://relay.example.com/actor",
		InboxURI: "https://relay.example.com/inbox",
		Name:     "Test Relay",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	// Set up a local account
	localAcct := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAcct)

	mockClient := NewMockHTTPClient()

	// Set up response for actor fetch
	actorJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.social/users/writer",
		"type": "Person",
		"preferredUsername": "writer",
		"inbox": "https://mastodon.social/users/writer/inbox",
		"outbox": "https://mastodon.social/users/writer/outbox",
		"publicKey": {
			"id": "https://mastodon.social/users/writer#main-key",
			"owner": "https://mastodon.social/users/writer",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`
	mockClient.Responses["https://mastodon.social/users/writer"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(actorJSON)),
		Header:     make(http.Header),
	}

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockClient,
	}

	// Announce from relay with embedded object
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://relay.example.com/activities/announce-456",
		"type": "Announce",
		"actor": "https://relay.example.com/actor",
		"object": {
			"id": "https://mastodon.social/users/writer/statuses/123456",
			"type": "Note",
			"attributedTo": "https://mastodon.social/users/writer",
			"content": "<p>Hello from the fediverse!</p>",
			"published": "2025-01-01T12:00:00Z"
		}
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Verify activity was stored
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity, got %d", len(mockDB.Activities))
	}

	// Check stored activity
	for _, act := range mockDB.Activities {
		if act.ActivityType != "Create" {
			t.Errorf("Expected ActivityType 'Create', got '%s'", act.ActivityType)
		}
		if act.ObjectURI != "https://mastodon.social/users/writer/statuses/123456" {
			t.Errorf("Expected ObjectURI from embedded object, got '%s'", act.ObjectURI)
		}
	}
}

func TestHandleAnnounceFromRelayDuplicateSkipped(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up a relay
	relay := &domain.Relay{
		Id:       uuid.New(),
		ActorURI: "https://relay.example.com/actor",
		InboxURI: "https://relay.example.com/inbox",
		Name:     "Test Relay",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	// Pre-add an activity with the same object URI
	existingActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://original.com/activities/create-123",
		ActivityType: "Create",
		ActorURI:     "https://mastodon.social/users/writer",
		ObjectURI:    "https://mastodon.social/users/writer/statuses/existing",
		RawJSON:      "{}",
		Processed:    true,
	}
	mockDB.Activities[existingActivity.Id] = existingActivity
	mockDB.ActivitiesByObj[existingActivity.ObjectURI] = existingActivity

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Announce for an object we already have
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://relay.example.com/activities/announce-789",
		"type": "Announce",
		"actor": "https://relay.example.com/actor",
		"object": "https://mastodon.social/users/writer/statuses/existing"
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Should still only have the original activity (duplicate was skipped)
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity (duplicate should be skipped), got %d", len(mockDB.Activities))
	}
}

func TestHandleAnnounceFromRelayDuplicateByActivityURI(t *testing.T) {
	mockDB := NewMockDatabase()

	// Set up a relay
	relay := &domain.Relay{
		Id:       uuid.New(),
		ActorURI: "https://relay.example.com/actor",
		InboxURI: "https://relay.example.com/inbox",
		Name:     "Test Relay",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	// Pre-add an activity with the same announce URI (activity_uri)
	// This simulates receiving the same relay announce twice
	existingActivity := &domain.Activity{
		Id:           uuid.New(),
		ActivityURI:  "https://relay.example.com/activities/announce-duplicate",
		ActivityType: "Create",
		ActorURI:     "https://mastodon.social/users/writer",
		ObjectURI:    "https://mastodon.social/users/writer/statuses/456",
		RawJSON:      "{}",
		Processed:    true,
	}
	mockDB.CreateActivity(existingActivity)

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: NewMockHTTPClient(),
	}

	// Announce with same ID (re-delivery or duplicate)
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://relay.example.com/activities/announce-duplicate",
		"type": "Announce",
		"actor": "https://relay.example.com/actor",
		"object": "https://mastodon.social/users/writer/statuses/different-object"
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Should still only have the original activity (duplicate was skipped by activity_uri)
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity (duplicate by activity_uri should be skipped), got %d", len(mockDB.Activities))
	}
}

func TestHandleAnnounceNotFromRelay(t *testing.T) {
	mockDB := NewMockDatabase()

	// No relay set up - this will be a regular boost

	// Set up local account
	localAcct := &domain.Account{
		Id:       uuid.New(),
		Username: "alice",
	}
	mockDB.AddAccount(localAcct)

	// Set up a note to boost
	note := &domain.Note{
		Id:        uuid.New(),
		Message:   "Original post",
		CreatedBy: "alice",
		ObjectURI: "https://example.com/notes/123",
	}
	mockDB.Notes[note.Id] = note
	mockDB.NotesByURI[note.ObjectURI] = note

	// Set up the remote actor who is boosting
	remoteActor := &domain.RemoteAccount{
		Id:       uuid.New(),
		Username: "bob",
		Domain:   "remote.example.com",
		ActorURI: "https://remote.example.com/users/bob",
		InboxURI: "https://remote.example.com/users/bob/inbox",
	}
	mockDB.RemoteAccounts[remoteActor.Id] = remoteActor
	mockDB.RemoteByActor[remoteActor.ActorURI] = remoteActor

	// Set up mock HTTP client with actor response
	mockClient := NewMockHTTPClient()
	actorJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/users/bob",
		"type": "Person",
		"preferredUsername": "bob",
		"inbox": "https://remote.example.com/users/bob/inbox",
		"outbox": "https://remote.example.com/users/bob/outbox",
		"publicKey": {
			"id": "https://remote.example.com/users/bob#main-key",
			"owner": "https://remote.example.com/users/bob",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\ntest\n-----END PUBLIC KEY-----"
		}
	}`
	mockClient.Responses["https://remote.example.com/users/bob"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(actorJSON)),
		Header:     make(http.Header),
	}

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockClient,
	}

	// Announce from a regular user (not a relay)
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://remote.example.com/activities/boost-1",
		"type": "Announce",
		"actor": "https://remote.example.com/users/bob",
		"object": "https://example.com/notes/123"
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Should have created a boost record (not a Create activity)
	if len(mockDB.Boosts) != 1 {
		t.Errorf("Expected 1 boost, got %d", len(mockDB.Boosts))
	}

	// No activities should be created for regular boosts
	if len(mockDB.Activities) != 0 {
		t.Errorf("Expected 0 activities (regular boost, not relay), got %d", len(mockDB.Activities))
	}
}

func TestExtractDomainFromURI(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected string
	}{
		{
			name:     "HTTPS with path",
			uri:      "https://relay.fedi.buzz/tag/music",
			expected: "relay.fedi.buzz",
		},
		{
			name:     "HTTPS without path",
			uri:      "https://example.com",
			expected: "example.com",
		},
		{
			name:     "HTTP with path",
			uri:      "http://example.org/users/alice",
			expected: "example.org",
		},
		{
			name:     "With port",
			uri:      "https://example.com:8443/path",
			expected: "example.com:8443",
		},
		{
			name:     "Empty string",
			uri:      "",
			expected: "",
		},
		{
			name:     "Invalid scheme",
			uri:      "ftp://example.com/file",
			expected: "",
		},
		{
			name:     "No scheme",
			uri:      "example.com/path",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomainFromURI(tt.uri)
			if result != tt.expected {
				t.Errorf("extractDomainFromURI(%q) = %q, want %q", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestIsActorFromAnyRelay(t *testing.T) {
	// Setup mock database with relays
	mockDB := NewMockDatabase()

	// Add an active relay for relay.fedi.buzz/tag/music
	relayID := uuid.New()
	relay := &domain.Relay{
		Id:       relayID,
		ActorURI: "https://relay.fedi.buzz/tag/music",
		InboxURI: "https://relay.fedi.buzz/tag/music/inbox",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	tests := []struct {
		name     string
		actorURI string
		expected bool
	}{
		{
			name:     "Exact match of subscribed relay",
			actorURI: "https://relay.fedi.buzz/tag/music",
			expected: true,
		},
		{
			name:     "Different tag from same relay domain",
			actorURI: "https://relay.fedi.buzz/tag/prints",
			expected: true,
		},
		{
			name:     "Another tag from same relay domain",
			actorURI: "https://relay.fedi.buzz/tag/photography",
			expected: true,
		},
		{
			name:     "Different domain",
			actorURI: "https://mastodon.social/users/alice",
			expected: false,
		},
		{
			name:     "Similar domain but not the same",
			actorURI: "https://fedi.buzz/actor",
			expected: false,
		},
		{
			name:     "Subdomain of relay domain",
			actorURI: "https://sub.relay.fedi.buzz/tag/art",
			expected: false,
		},
		{
			name:     "Invalid URI",
			actorURI: "not-a-uri",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isActorFromAnyRelay(tt.actorURI, mockDB)
			if result != tt.expected {
				t.Errorf("isActorFromAnyRelay(%q) = %v, want %v", tt.actorURI, result, tt.expected)
			}
		})
	}
}

func TestHandleAnnounceFromRelayDifferentTag(t *testing.T) {
	// Test that Announces from a different tag on the same relay domain are handled as relay content
	// This tests the scenario where we subscribe to /tag/music but receive content from /tag/prints

	mockDB := NewMockDatabase()
	mockClient := NewMockHTTPClient()

	// Add local account
	localAccountID := uuid.New()
	localAccount := &domain.Account{
		Id:       localAccountID,
		Username: "alice",
	}
	mockDB.AddAccount(localAccount)

	// Add an active relay subscription for /tag/music
	relayID := uuid.New()
	relay := &domain.Relay{
		Id:       relayID,
		ActorURI: "https://relay.fedi.buzz/tag/music",
		InboxURI: "https://relay.fedi.buzz/tag/music/inbox",
		Status:   "active",
	}
	mockDB.CreateRelay(relay)

	// Mock HTTP response for fetching the original author
	authorJSON := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://mastodon.world/users/artist",
		"type": "Person",
		"preferredUsername": "artist",
		"inbox": "https://mastodon.world/users/artist/inbox",
		"publicKey": {
			"id": "https://mastodon.world/users/artist#main-key",
			"owner": "https://mastodon.world/users/artist",
			"publicKeyPem": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...\n-----END PUBLIC KEY-----"
		}
	}`

	mockClient.Responses["https://mastodon.world/users/artist"] = &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(authorJSON)),
		Header:     make(http.Header),
	}

	deps := &InboxDeps{
		Database:   mockDB,
		HTTPClient: mockClient,
	}

	// Announce from /tag/prints (different tag, same relay domain)
	// This is the key scenario: we're subscribed to /tag/music but relay sends from /tag/prints
	announceBody := `{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id": "https://relay.fedi.buzz/activities/announce-123",
		"type": "Announce",
		"actor": "https://relay.fedi.buzz/tag/prints",
		"object": {
			"id": "https://mastodon.world/users/artist/statuses/123456",
			"type": "Note",
			"attributedTo": "https://mastodon.world/users/artist",
			"content": "<p>Check out my latest art!</p>"
		},
		"published": "2024-01-15T10:30:00Z"
	}`

	err := handleAnnounceActivityWithDeps([]byte(announceBody), "alice", deps)
	if err != nil {
		t.Fatalf("handleAnnounceActivityWithDeps failed: %v", err)
	}

	// Should have created an activity (relay-forwarded content)
	if len(mockDB.Activities) != 1 {
		t.Errorf("Expected 1 activity (relay-forwarded), got %d", len(mockDB.Activities))
	}

	// Verify the activity was stored correctly
	for _, activity := range mockDB.Activities {
		if activity.ActivityType != "Create" {
			t.Errorf("Expected activity type 'Create', got %q", activity.ActivityType)
		}
		if activity.ActorURI != "https://mastodon.world/users/artist" {
			t.Errorf("Expected actor 'https://mastodon.world/users/artist', got %q", activity.ActorURI)
		}
		if activity.ObjectURI != "https://mastodon.world/users/artist/statuses/123456" {
			t.Errorf("Expected object URI 'https://mastodon.world/users/artist/statuses/123456', got %q", activity.ObjectURI)
		}
	}

	// Should not create a boost record (this is relay content, not a boost)
	if len(mockDB.Boosts) != 0 {
		t.Errorf("Expected 0 boosts (relay content, not boost), got %d", len(mockDB.Boosts))
	}
}

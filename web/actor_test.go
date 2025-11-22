package web

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/deemkeen/stegodon/util"
)

func TestGetFollowersCollection(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	tests := []struct {
		name         string
		actor        string
		followerURIs []string
		wantCount    int
	}{
		{
			name:         "Empty followers list",
			actor:        "alice",
			followerURIs: []string{},
			wantCount:    0,
		},
		{
			name:  "Single follower",
			actor: "bob",
			followerURIs: []string{
				"https://mastodon.social/users/charlie",
			},
			wantCount: 1,
		},
		{
			name:  "Multiple followers",
			actor: "carol",
			followerURIs: []string{
				"https://mastodon.social/users/alice",
				"https://pleroma.example/users/bob",
				"https://example.com/users/dave",
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFollowersCollection(tt.actor, conf, tt.followerURIs)

			// Parse the JSON result
			var collection map[string]any
			if err := json.Unmarshal([]byte(result), &collection); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Verify @context
			if collection["@context"] != "https://www.w3.org/ns/activitystreams" {
				t.Errorf("Expected @context to be ActivityStreams, got: %v", collection["@context"])
			}

			// Verify type
			if collection["type"] != "OrderedCollection" {
				t.Errorf("Expected type to be OrderedCollection, got: %v", collection["type"])
			}

			// Verify id
			expectedID := "https://example.com/users/" + tt.actor + "/followers"
			if collection["id"] != expectedID {
				t.Errorf("Expected id to be %s, got: %v", expectedID, collection["id"])
			}

			// Verify totalItems
			totalItems := int(collection["totalItems"].(float64))
			if totalItems != tt.wantCount {
				t.Errorf("Expected totalItems to be %d, got: %d", tt.wantCount, totalItems)
			}

			// Verify first link is present (always uses paging now)
			expectedFirst := fmt.Sprintf("https://example.com/users/%s/followers?page=1", tt.actor)
			if first, ok := collection["first"].(string); !ok || first != expectedFirst {
				t.Errorf("Expected first to be %s, got: %v", expectedFirst, collection["first"])
			}

			// Verify orderedItems is NOT present (should use paging)
			if _, exists := collection["orderedItems"]; exists {
				t.Error("Collections should use paging with 'first' link, not inline orderedItems")
			}
		})
	}
}

func TestGetFollowingCollection(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	tests := []struct {
		name          string
		actor         string
		followingURIs []string
		wantCount     int
	}{
		{
			name:          "Empty following list",
			actor:         "alice",
			followingURIs: []string{},
			wantCount:     0,
		},
		{
			name:  "Single following",
			actor: "bob",
			followingURIs: []string{
				"https://mastodon.social/users/charlie",
			},
			wantCount: 1,
		},
		{
			name:  "Multiple following",
			actor: "carol",
			followingURIs: []string{
				"https://mastodon.social/users/alice",
				"https://pleroma.example/users/bob",
				"https://example.com/users/dave",
			},
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetFollowingCollection(tt.actor, conf, tt.followingURIs)

			// Parse the JSON result
			var collection map[string]any
			if err := json.Unmarshal([]byte(result), &collection); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			// Verify @context
			if collection["@context"] != "https://www.w3.org/ns/activitystreams" {
				t.Errorf("Expected @context to be ActivityStreams, got: %v", collection["@context"])
			}

			// Verify type
			if collection["type"] != "OrderedCollection" {
				t.Errorf("Expected type to be OrderedCollection, got: %v", collection["type"])
			}

			// Verify id
			expectedID := "https://example.com/users/" + tt.actor + "/following"
			if collection["id"] != expectedID {
				t.Errorf("Expected id to be %s, got: %v", expectedID, collection["id"])
			}

			// Verify totalItems
			totalItems := int(collection["totalItems"].(float64))
			if totalItems != tt.wantCount {
				t.Errorf("Expected totalItems to be %d, got: %d", tt.wantCount, totalItems)
			}

			// Verify first link is present (always uses paging now)
			expectedFirst := fmt.Sprintf("https://example.com/users/%s/following?page=1", tt.actor)
			if first, ok := collection["first"].(string); !ok || first != expectedFirst {
				t.Errorf("Expected first to be %s, got: %v", expectedFirst, collection["first"])
			}

			// Verify orderedItems is NOT present (should use paging)
			if _, exists := collection["orderedItems"]; exists {
				t.Error("Collections should use paging with 'first' link, not inline orderedItems")
			}
		})
	}
}

func TestGetFollowersCollection_ActivityPubCompliance(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	followerURIs := []string{
		"https://mastodon.social/users/alice",
		"https://pleroma.example/users/bob",
	}

	result := GetFollowersCollection("testuser", conf, followerURIs)

	// Parse as generic JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify required ActivityPub properties exist (with paging)
	requiredFields := []string{"@context", "id", "type", "totalItems", "first"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Verify type is OrderedCollection (ActivityPub spec)
	if data["type"] != "OrderedCollection" {
		t.Errorf("Type must be OrderedCollection for ActivityPub compliance")
	}

	// Verify first link is a string
	if _, ok := data["first"].(string); !ok {
		t.Error("first must be a string URL")
	}

	// Verify orderedItems is NOT present (using paging)
	if _, exists := data["orderedItems"]; exists {
		t.Error("orderedItems should not be present when using paging")
	}
}

func TestGetFollowingCollection_ActivityPubCompliance(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	followingURIs := []string{
		"https://mastodon.social/users/alice",
		"https://pleroma.example/users/bob",
	}

	result := GetFollowingCollection("testuser", conf, followingURIs)

	// Parse as generic JSON
	var data map[string]any
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify required ActivityPub properties exist (with paging)
	requiredFields := []string{"@context", "id", "type", "totalItems", "first"}
	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Verify type is OrderedCollection (ActivityPub spec)
	if data["type"] != "OrderedCollection" {
		t.Errorf("Type must be OrderedCollection for ActivityPub compliance")
	}

	// Verify first link is a string
	if _, ok := data["first"].(string); !ok {
		t.Error("first must be a string URL")
	}

	// Verify orderedItems is NOT present (using paging)
	if _, exists := data["orderedItems"]; exists {
		t.Error("orderedItems should not be present when using paging")
	}
}

func TestGetFollowersPage(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	followerURIs := []string{
		"https://mastodon.social/users/alice",
		"https://pleroma.example/users/bob",
		"https://example.com/users/charlie",
	}

	result := GetFollowersPage("testuser", conf, followerURIs, 1)

	var page map[string]any
	if err := json.Unmarshal([]byte(result), &page); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify @context
	if page["@context"] != "https://www.w3.org/ns/activitystreams" {
		t.Errorf("Expected @context to be ActivityStreams, got: %v", page["@context"])
	}

	// Verify type is OrderedCollectionPage
	if page["type"] != "OrderedCollectionPage" {
		t.Errorf("Expected type to be OrderedCollectionPage, got: %v", page["type"])
	}

	// Verify id
	expectedID := "https://example.com/users/testuser/followers?page=1"
	if page["id"] != expectedID {
		t.Errorf("Expected id to be %s, got: %v", expectedID, page["id"])
	}

	// Verify partOf
	expectedPartOf := "https://example.com/users/testuser/followers"
	if page["partOf"] != expectedPartOf {
		t.Errorf("Expected partOf to be %s, got: %v", expectedPartOf, page["partOf"])
	}

	// Verify orderedItems is present
	orderedItems, ok := page["orderedItems"].([]any)
	if !ok {
		t.Fatal("Expected orderedItems to be an array")
	}

	if len(orderedItems) != 3 {
		t.Errorf("Expected 3 items in orderedItems, got: %d", len(orderedItems))
	}

	// Verify each URI
	for i, expectedURI := range followerURIs {
		actualURI := orderedItems[i].(string)
		if actualURI != expectedURI {
			t.Errorf("Expected orderedItems[%d] to be %s, got: %s", i, expectedURI, actualURI)
		}
	}
}

func TestGetFollowingPage(t *testing.T) {
	conf := &util.AppConfig{}
	conf.Conf.SslDomain = "example.com"

	followingURIs := []string{
		"https://mastodon.social/users/alice",
		"https://pleroma.example/users/bob",
	}

	result := GetFollowingPage("testuser", conf, followingURIs, 1)

	var page map[string]any
	if err := json.Unmarshal([]byte(result), &page); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify @context
	if page["@context"] != "https://www.w3.org/ns/activitystreams" {
		t.Errorf("Expected @context to be ActivityStreams, got: %v", page["@context"])
	}

	// Verify type is OrderedCollectionPage
	if page["type"] != "OrderedCollectionPage" {
		t.Errorf("Expected type to be OrderedCollectionPage, got: %v", page["type"])
	}

	// Verify id
	expectedID := "https://example.com/users/testuser/following?page=1"
	if page["id"] != expectedID {
		t.Errorf("Expected id to be %s, got: %v", expectedID, page["id"])
	}

	// Verify partOf
	expectedPartOf := "https://example.com/users/testuser/following"
	if page["partOf"] != expectedPartOf {
		t.Errorf("Expected partOf to be %s, got: %v", expectedPartOf, page["partOf"])
	}

	// Verify orderedItems is present
	orderedItems, ok := page["orderedItems"].([]any)
	if !ok {
		t.Fatal("Expected orderedItems to be an array")
	}

	if len(orderedItems) != 2 {
		t.Errorf("Expected 2 items in orderedItems, got: %d", len(orderedItems))
	}

	// Verify each URI
	for i, expectedURI := range followingURIs {
		actualURI := orderedItems[i].(string)
		if actualURI != expectedURI {
			t.Errorf("Expected orderedItems[%d] to be %s, got: %s", i, expectedURI, actualURI)
		}
	}
}

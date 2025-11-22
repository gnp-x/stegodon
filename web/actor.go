package web

import (
	"encoding/json"
	"fmt"
	"time"

	"strings"

	"github.com/deemkeen/stegodon/db"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

type action uint

const (
	id action = iota
	inbox
	outbox
	followers
	following
	sharedInbox
)

func GetActor(actor string, conf *util.AppConfig) (error, string) {
	err, acc := db.GetDB().ReadAccByUsername(actor)
	if err != nil {
		return err, "{}"
	}

	username := acc.Username
	pubKey := strings.ReplaceAll(acc.WebPublicKey, "\n", "\\n")

	// Use DisplayName if available, otherwise use username
	displayName := acc.DisplayName
	if displayName == "" {
		displayName = username
	}

	// Escape any quotes in summary for JSON
	summary := strings.ReplaceAll(acc.Summary, "\"", "\\\"")
	summary = strings.ReplaceAll(summary, "\n", "\\n")

	// Use default logo for all users
	logoURL := fmt.Sprintf("https://%s/static/stegologo.png", conf.Conf.SslDomain)

	return nil, fmt.Sprintf(
		`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],

					"id": "%s",
					"type": "Person",
					"preferredUsername": "%s",
					"name" : "%s",
					"summary": "%s",
					"inbox": "%s",
					"outbox": "%s",
					"followers": "%s",
					"following": "%s",
					"url": "%s",
  					"manuallyApprovesFollowers": false,
					"discoverable": true,
					"icon": {
						"type": "Image",
						"mediaType": "image/png",
						"url": "%s"
					},
  					"endpoints": {
    					"sharedInbox": "%s"
  					},
					"publicKey": {
						"id": "%s#main-key",
						"owner": "%s",
						"publicKeyPem": "%s"
					}
				}`,
		getIRI(conf.Conf.SslDomain, username, id),
		username, displayName, summary,
		getIRI(conf.Conf.SslDomain, username, inbox),
		getIRI(conf.Conf.SslDomain, username, outbox),
		getIRI(conf.Conf.SslDomain, username, followers),
		getIRI(conf.Conf.SslDomain, username, following),
		getIRI(conf.Conf.SslDomain, username, id),
		logoURL,
		getIRI(conf.Conf.SslDomain, username, sharedInbox),
		getIRI(conf.Conf.SslDomain, username, id),
		getIRI(conf.Conf.SslDomain, username, id), pubKey)
}

func getIRI(domain string, username string, action action) string {

	prefix := fmt.Sprintf("https://%s/users/%s", domain, username)
	switch action {
	case inbox:
		return fmt.Sprintf("%s/inbox", prefix)
	case outbox:
		return fmt.Sprintf("%s/outbox", prefix)
	case followers:
		return fmt.Sprintf("%s/followers", prefix)
	case following:
		return fmt.Sprintf("%s/following", prefix)
	case id:
		return prefix
	case sharedInbox:
		return fmt.Sprintf("https://%s/inbox", domain)
	default:
		return ""
	}
}

// GetNoteObject returns a Note object as ActivityPub JSON
func GetNoteObject(noteId uuid.UUID, conf *util.AppConfig) (error, string) {
	database := db.GetDB()
	err, note := database.ReadNoteId(noteId)
	if err != nil {
		return err, "{}"
	}

	// Get the account to build actor URI
	err, account := database.ReadAccByUsername(note.CreatedBy)
	if err != nil {
		return err, "{}"
	}

	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, account.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())

	// Convert Markdown links to HTML for ActivityPub content
	contentHTML := util.MarkdownLinksToHTML(note.Message)

	// Build the Note object
	noteObj := map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           noteURI,
		"type":         "Note",
		"attributedTo": actorURI,
		"content":      contentHTML,
		"mediaType":    "text/html",
		"published":    note.CreatedAt.Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, account.Username),
		},
	}

	// Add updated field if note was edited
	if note.EditedAt != nil {
		noteObj["updated"] = note.EditedAt.Format(time.RFC3339)
	}

	jsonBytes, err := json.Marshal(noteObj)
	if err != nil {
		return err, "{}"
	}

	return nil, string(jsonBytes)
}

// GetFollowersCollection returns an ActivityPub OrderedCollection of followers
// Always uses paging for compatibility with Mastodon and other servers
func GetFollowersCollection(actor string, conf *util.AppConfig, followerURIs []string) string {
	collectionURI := fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, actor)

	// Always use paging (Mastodon expects this)
	collection := map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         collectionURI,
		"type":       "OrderedCollection",
		"totalItems": len(followerURIs),
		"first":      fmt.Sprintf("%s?page=1", collectionURI),
	}

	jsonBytes, err := json.Marshal(collection)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// GetFollowingCollection returns an ActivityPub OrderedCollection of following
// Always uses paging for compatibility with Mastodon and other servers
func GetFollowingCollection(actor string, conf *util.AppConfig, followingURIs []string) string {
	collectionURI := fmt.Sprintf("https://%s/users/%s/following", conf.Conf.SslDomain, actor)

	// Always use paging (Mastodon expects this)
	collection := map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         collectionURI,
		"type":       "OrderedCollection",
		"totalItems": len(followingURIs),
		"first":      fmt.Sprintf("%s?page=1", collectionURI),
	}

	jsonBytes, err := json.Marshal(collection)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// GetFollowersPage returns an OrderedCollectionPage for followers
func GetFollowersPage(actor string, conf *util.AppConfig, followerURIs []string, page int) string {
	collectionURI := fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, actor)
	pageURI := fmt.Sprintf("%s?page=%d", collectionURI, page)

	collectionPage := map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           pageURI,
		"type":         "OrderedCollectionPage",
		"partOf":       collectionURI,
		"orderedItems": followerURIs,
		"totalItems":   len(followerURIs),
	}

	jsonBytes, err := json.Marshal(collectionPage)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// GetFollowingPage returns an OrderedCollectionPage for following
func GetFollowingPage(actor string, conf *util.AppConfig, followingURIs []string, page int) string {
	collectionURI := fmt.Sprintf("https://%s/users/%s/following", conf.Conf.SslDomain, actor)
	pageURI := fmt.Sprintf("%s?page=%d", collectionURI, page)

	collectionPage := map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           pageURI,
		"type":         "OrderedCollectionPage",
		"partOf":       collectionURI,
		"orderedItems": followingURIs,
		"totalItems":   len(followingURIs),
	}

	jsonBytes, err := json.Marshal(collectionPage)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

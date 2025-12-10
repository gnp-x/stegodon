package activitypub

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/deemkeen/stegodon/util"
	"github.com/google/uuid"
)

// SendActivity sends an activity to a remote inbox.
// This is the production wrapper that uses the default HTTP client.
func SendActivity(activity any, inboxURI string, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendActivityWithDeps(activity, inboxURI, localAccount, conf, defaultHTTPClient)
}

// SendActivityWithDeps sends an activity to a remote inbox.
// This version accepts dependencies for testing.
func SendActivityWithDeps(activity any, inboxURI string, localAccount *domain.Account, conf *util.AppConfig, client HTTPClient) error {
	// Marshal activity to JSON
	activityJSON, err := json.Marshal(activity)
	if err != nil {
		return fmt.Errorf("failed to marshal activity: %w", err)
	}

	// Calculate digest for HTTP signature
	hash := sha256.Sum256(activityJSON)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(hash[:])

	// Create HTTP request
	req, err := http.NewRequest("POST", inboxURI, bytes.NewReader(activityJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/activity+json")
	req.Header.Set("Accept", "application/activity+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("Digest", digest)

	// Parse private key for signing
	privateKey, err := ParsePrivateKey(localAccount.WebPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	// Sign request
	keyID := fmt.Sprintf("https://%s/users/%s#main-key", conf.Conf.SslDomain, localAccount.Username)
	if err := SignRequest(req, privateKey, keyID); err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("remote server returned status: %d", resp.StatusCode)
	}

	log.Printf("Outbox: Sent %T to %s (status: %d)", activity, inboxURI, resp.StatusCode)
	return nil
}

// SendAccept sends an Accept activity in response to a Follow.
// This is the production wrapper that uses the default HTTP client.
func SendAccept(localAccount *domain.Account, remoteActor *domain.RemoteAccount, followID string, conf *util.AppConfig) error {
	return SendAcceptWithDeps(localAccount, remoteActor, followID, conf, defaultHTTPClient)
}

// SendAcceptWithDeps sends an Accept activity in response to a Follow.
// This version accepts dependencies for testing.
func SendAcceptWithDeps(localAccount *domain.Account, remoteActor *domain.RemoteAccount, followID string, conf *util.AppConfig, client HTTPClient) error {
	acceptID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	accept := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       acceptID,
		"type":     "Accept",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     followID,
			"type":   "Follow",
			"actor":  remoteActor.ActorURI,
			"object": actorURI,
		},
	}

	return SendActivityWithDeps(accept, remoteActor.InboxURI, localAccount, conf, client)
}

// SendCreate sends a Create activity for a new note.
// This is the production wrapper that uses the default database.
func SendCreate(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendCreateWithDeps(note, localAccount, conf, NewDBWrapper())
}

// SendCreateWithDeps sends a Create activity for a new note.
// This version accepts dependencies for testing.
func SendCreateWithDeps(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())
	createID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)

	// Convert Markdown links to HTML for ActivityPub content
	contentHTML := util.MarkdownLinksToHTML(note.Message)
	// Convert hashtags to ActivityPub-compliant HTML links
	contentHTML = util.HashtagsToActivityPubHTML(contentHTML, baseURL)

	// Build cc list - start with followers
	ccList := []string{
		fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
	}

	// If this is a reply, add the parent author to cc for delivery
	var parentAuthorURI string
	if note.InReplyToURI != "" {
		// Try to extract parent author from the inReplyToURI or fetch it
		parentAuthorURI = extractAuthorFromURI(note.InReplyToURI, database, conf)
		if parentAuthorURI != "" && parentAuthorURI != actorURI {
			ccList = append(ccList, parentAuthorURI)
		}
	}

	// Build the Note object
	noteObj := map[string]any{
		"id":           noteURI,
		"type":         "Note",
		"attributedTo": actorURI,
		"content":      contentHTML,
		"mediaType":    "text/html",
		"published":    note.CreatedAt.Format(time.RFC3339),
		"url":          fmt.Sprintf("https://%s/u/%s/%s", conf.Conf.SslDomain, localAccount.Username, note.Id.String()),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": ccList,
	}

	// Add inReplyTo if this is a reply
	if note.InReplyToURI != "" {
		noteObj["inReplyTo"] = note.InReplyToURI
		log.Printf("Outbox: Note %s is a reply to %s", note.Id, note.InReplyToURI)
	}

	// Extract hashtags and add to tag array
	hashtags := util.ParseHashtags(note.Message)
	tags := make([]map[string]any, 0)

	for _, tag := range hashtags {
		tags = append(tags, map[string]any{
			"type": "Hashtag",
			"href": fmt.Sprintf("https://%s/tags/%s", conf.Conf.SslDomain, tag),
			"name": "#" + tag,
		})
	}

	// Extract mentions and resolve actor URIs
	mentions := util.ParseMentions(note.Message)
	mentionURIs := make(map[string]string)
	mentionedActors := make([]string, 0)

	for _, mention := range mentions {
		// Skip local mentions (same domain) - they don't need federation
		if strings.EqualFold(mention.Domain, conf.Conf.SslDomain) {
			continue
		}

		// Resolve via WebFinger
		actorURI, err := resolveMentionURI(mention.Username, mention.Domain)
		if err != nil {
			log.Printf("Outbox: Failed to resolve mention @%s@%s: %v", mention.Username, mention.Domain, err)
			continue
		}

		mentionKey := fmt.Sprintf("@%s@%s", mention.Username, mention.Domain)
		mentionURIs[mentionKey] = actorURI
		mentionedActors = append(mentionedActors, actorURI)

		tags = append(tags, map[string]any{
			"type": "Mention",
			"href": actorURI,
			"name": mentionKey,
		})
	}

	if len(tags) > 0 {
		noteObj["tag"] = tags
	}

	// Add mentioned actors to cc list for proper ActivityPub addressing
	for _, mentionActorURI := range mentionedActors {
		ccList = append(ccList, mentionActorURI)
	}
	// Update noteObj cc with the expanded list
	noteObj["cc"] = ccList

	// Convert mentions to ActivityPub HTML (after we have resolved URIs)
	if len(mentionURIs) > 0 {
		contentHTML = util.MentionsToActivityPubHTML(contentHTML, mentionURIs)
		noteObj["content"] = contentHTML
	}

	// Build context - include Hashtag definition if we have hashtags
	var context any
	if len(hashtags) > 0 {
		context = []any{
			"https://www.w3.org/ns/activitystreams",
			map[string]any{
				"Hashtag": "as:Hashtag",
			},
		}
	} else {
		context = "https://www.w3.org/ns/activitystreams"
	}

	create := map[string]any{
		"@context":  context,
		"id":        createID,
		"type":      "Create",
		"actor":     actorURI,
		"published": note.CreatedAt.Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc":     ccList,
		"object": noteObj,
	}

	// Collect inboxes to deliver to (followers + parent author for replies)
	inboxes := make(map[string]bool) // Use map to dedupe

	// Get all followers
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers: %v", err)
	} else if followers != nil {
		for _, follower := range *followers {
			// Skip local followers - they don't need federation delivery
			if follower.IsLocal {
				continue
			}
			err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
			if err != nil {
				log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
				continue
			}
			inboxes[remoteActor.InboxURI] = true
		}
	}

	// If this is a reply, also deliver to the parent author's inbox
	if parentAuthorURI != "" && parentAuthorURI != actorURI {
		// First try as remote account
		err, parentAccount := database.ReadRemoteAccountByActorURI(parentAuthorURI)
		if err == nil && parentAccount != nil {
			inboxes[parentAccount.InboxURI] = true
			log.Printf("Outbox: Will also deliver reply to remote parent author %s@%s", parentAccount.Username, parentAccount.Domain)
		} else {
			// Try as local account - extract username from URI like https://domain/users/username
			if strings.Contains(parentAuthorURI, conf.Conf.SslDomain) {
				parts := strings.Split(parentAuthorURI, "/users/")
				if len(parts) == 2 {
					parentUsername := parts[1]
					// Verify this local user exists
					err, localParent := database.ReadAccByUsername(parentUsername)
					if err == nil && localParent != nil {
						// Construct local inbox URI
						localInboxURI := fmt.Sprintf("https://%s/users/%s/inbox", conf.Conf.SslDomain, parentUsername)
						inboxes[localInboxURI] = true
						log.Printf("Outbox: Will also deliver reply to local parent author %s", parentUsername)
					}
				}
			}
		}
	}

	// Also deliver to mentioned actors' inboxes
	for _, mentionActorURI := range mentionedActors {
		// Look up the remote account to get their inbox
		err, mentionedAccount := database.ReadRemoteAccountByActorURI(mentionActorURI)
		if err == nil && mentionedAccount != nil {
			inboxes[mentionedAccount.InboxURI] = true
			log.Printf("Outbox: Will also deliver to mentioned user %s@%s", mentionedAccount.Username, mentionedAccount.Domain)
		} else {
			// Fetch the actor if not cached
			mentionedAccount, err = FetchRemoteActorWithDeps(mentionActorURI, defaultHTTPClient, database)
			if err == nil && mentionedAccount != nil {
				inboxes[mentionedAccount.InboxURI] = true
				log.Printf("Outbox: Will also deliver to mentioned user %s@%s (fetched)", mentionedAccount.Username, mentionedAccount.Domain)
			} else {
				log.Printf("Outbox: Could not resolve inbox for mentioned actor %s: %v", mentionActorURI, err)
			}
		}
	}

	// Get active relays and add their inboxes
	err, relays := database.ReadActiveRelays()
	if err == nil && relays != nil {
		for _, relay := range *relays {
			inboxes[relay.InboxURI] = true
			log.Printf("Outbox: Will also deliver to relay %s", relay.ActorURI)
		}
	}

	if len(inboxes) == 0 {
		log.Printf("Outbox: No inboxes to deliver to")
		return nil
	}

	// Queue delivery to each unique inbox
	for inboxURI := range inboxes {
		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     inboxURI,
			ActivityJSON: mustMarshal(create),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue delivery to %s: %v", inboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Create activity for note %s to %d inboxes", note.Id, len(inboxes))
	return nil
}

// SendUpdate sends an Update activity to all followers when a note is edited.
// This is the production wrapper that uses the default database.
func SendUpdate(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendUpdateWithDeps(note, localAccount, conf, NewDBWrapper())
}

// SendUpdateWithDeps sends an Update activity to all followers when a note is edited.
// This version accepts dependencies for testing.
func SendUpdateWithDeps(note *domain.Note, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, note.Id.String())
	updateID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	baseURL := fmt.Sprintf("https://%s", conf.Conf.SslDomain)

	// Use EditedAt if available, otherwise use CreatedAt
	updatedTime := note.CreatedAt
	if note.EditedAt != nil {
		updatedTime = *note.EditedAt
	}

	// Convert Markdown links to HTML for ActivityPub content
	contentHTML := util.MarkdownLinksToHTML(note.Message)
	// Convert hashtags to ActivityPub-compliant HTML links
	contentHTML = util.HashtagsToActivityPubHTML(contentHTML, baseURL)

	// Build cc list - start with followers
	ccList := []string{
		fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
	}

	// If this is a reply, add the parent author to cc for delivery
	var parentAuthorURI string
	if note.InReplyToURI != "" {
		parentAuthorURI = extractAuthorFromURI(note.InReplyToURI, database, conf)
		if parentAuthorURI != "" && parentAuthorURI != actorURI {
			ccList = append(ccList, parentAuthorURI)
		}
	}

	// Build the Note object
	noteObj := map[string]any{
		"id":           noteURI,
		"type":         "Note",
		"attributedTo": actorURI,
		"content":      contentHTML,
		"mediaType":    "text/html",
		"published":    note.CreatedAt.Format(time.RFC3339),
		"updated":      updatedTime.Format(time.RFC3339),
		"url":          fmt.Sprintf("https://%s/u/%s/%s", conf.Conf.SslDomain, localAccount.Username, note.Id.String()),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": ccList,
	}

	// Add inReplyTo if this is a reply
	if note.InReplyToURI != "" {
		noteObj["inReplyTo"] = note.InReplyToURI
	}

	// Extract hashtags and add to tag array
	hashtags := util.ParseHashtags(note.Message)
	tags := make([]map[string]any, 0)

	for _, tag := range hashtags {
		tags = append(tags, map[string]any{
			"type": "Hashtag",
			"href": fmt.Sprintf("https://%s/tags/%s", conf.Conf.SslDomain, tag),
			"name": "#" + tag,
		})
	}

	// Extract mentions and resolve actor URIs
	mentions := util.ParseMentions(note.Message)
	mentionURIs := make(map[string]string)
	mentionedActors := make([]string, 0)

	for _, mention := range mentions {
		// Skip local mentions (same domain) - they don't need federation
		if strings.EqualFold(mention.Domain, conf.Conf.SslDomain) {
			continue
		}

		// Resolve via WebFinger
		actorURI, err := resolveMentionURI(mention.Username, mention.Domain)
		if err != nil {
			log.Printf("Outbox: Failed to resolve mention @%s@%s: %v", mention.Username, mention.Domain, err)
			continue
		}

		mentionKey := fmt.Sprintf("@%s@%s", mention.Username, mention.Domain)
		mentionURIs[mentionKey] = actorURI
		mentionedActors = append(mentionedActors, actorURI)

		tags = append(tags, map[string]any{
			"type": "Mention",
			"href": actorURI,
			"name": mentionKey,
		})
	}

	if len(tags) > 0 {
		noteObj["tag"] = tags
	}

	// Add mentioned actors to cc list for proper ActivityPub addressing
	for _, mentionActorURI := range mentionedActors {
		ccList = append(ccList, mentionActorURI)
	}
	// Update noteObj cc with the expanded list
	noteObj["cc"] = ccList

	// Convert mentions to ActivityPub HTML (after we have resolved URIs)
	if len(mentionURIs) > 0 {
		contentHTML = util.MentionsToActivityPubHTML(contentHTML, mentionURIs)
		noteObj["content"] = contentHTML
	}

	// Build context - include Hashtag definition if we have hashtags
	var context any
	if len(hashtags) > 0 {
		context = []any{
			"https://www.w3.org/ns/activitystreams",
			map[string]any{
				"Hashtag": "as:Hashtag",
			},
		}
	} else {
		context = "https://www.w3.org/ns/activitystreams"
	}

	update := map[string]any{
		"@context": context,
		"id":       updateID,
		"type":     "Update",
		"actor":    actorURI,
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc":     ccList,
		"object": noteObj,
	}

	// Collect inboxes to deliver to (followers + parent author for replies)
	inboxes := make(map[string]bool)

	// Get all followers
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers for Update: %v", err)
	} else if followers != nil {
		for _, follower := range *followers {
			// Skip local followers - they don't need federation delivery
			if follower.IsLocal {
				continue
			}
			err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
			if err != nil {
				log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
				continue
			}
			inboxes[remoteActor.InboxURI] = true
		}
	}

	// If this is a reply, also deliver to the parent author's inbox
	if parentAuthorURI != "" && parentAuthorURI != actorURI {
		// First try as remote account
		err, parentAccount := database.ReadRemoteAccountByActorURI(parentAuthorURI)
		if err == nil && parentAccount != nil {
			inboxes[parentAccount.InboxURI] = true
		} else {
			// Try as local account - extract username from URI like https://domain/users/username
			if strings.Contains(parentAuthorURI, conf.Conf.SslDomain) {
				parts := strings.Split(parentAuthorURI, "/users/")
				if len(parts) == 2 {
					parentUsername := parts[1]
					// Verify this local user exists
					err, localParent := database.ReadAccByUsername(parentUsername)
					if err == nil && localParent != nil {
						// Construct local inbox URI
						localInboxURI := fmt.Sprintf("https://%s/users/%s/inbox", conf.Conf.SslDomain, parentUsername)
						inboxes[localInboxURI] = true
					}
				}
			}
		}
	}

	// Also deliver to mentioned actors' inboxes
	for _, mentionActorURI := range mentionedActors {
		// Look up the remote account to get their inbox
		err, mentionedAccount := database.ReadRemoteAccountByActorURI(mentionActorURI)
		if err == nil && mentionedAccount != nil {
			inboxes[mentionedAccount.InboxURI] = true
			log.Printf("Outbox: Will also deliver Update to mentioned user %s@%s", mentionedAccount.Username, mentionedAccount.Domain)
		} else {
			// Fetch the actor if not cached
			mentionedAccount, err = FetchRemoteActorWithDeps(mentionActorURI, defaultHTTPClient, database)
			if err == nil && mentionedAccount != nil {
				inboxes[mentionedAccount.InboxURI] = true
				log.Printf("Outbox: Will also deliver Update to mentioned user %s@%s (fetched)", mentionedAccount.Username, mentionedAccount.Domain)
			} else {
				log.Printf("Outbox: Could not resolve inbox for mentioned actor %s: %v", mentionActorURI, err)
			}
		}
	}

	// Get active relays and add their inboxes
	err, relays := database.ReadActiveRelays()
	if err == nil && relays != nil {
		for _, relay := range *relays {
			inboxes[relay.InboxURI] = true
			log.Printf("Outbox: Will also deliver Update to relay %s", relay.ActorURI)
		}
	}

	if len(inboxes) == 0 {
		log.Printf("Outbox: No inboxes to deliver Update to")
		return nil
	}

	// Queue delivery to each unique inbox
	for inboxURI := range inboxes {
		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     inboxURI,
			ActivityJSON: mustMarshal(update),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue Update delivery to %s: %v", inboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Update activity for note %s to %d inboxes", note.Id, len(inboxes))
	return nil
}

// SendDelete sends a Delete activity to all followers when a note is deleted.
// This is the production wrapper that uses the default database.
func SendDelete(noteId uuid.UUID, localAccount *domain.Account, conf *util.AppConfig) error {
	return SendDeleteWithDeps(noteId, localAccount, conf, NewDBWrapper())
}

// SendDeleteWithDeps sends a Delete activity to all followers when a note is deleted.
// This version accepts dependencies for testing.
func SendDeleteWithDeps(noteId uuid.UUID, localAccount *domain.Account, conf *util.AppConfig, database Database) error {
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)
	noteURI := fmt.Sprintf("https://%s/notes/%s", conf.Conf.SslDomain, noteId.String())
	deleteID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())

	deleteActivity := map[string]any{
		"@context":  "https://www.w3.org/ns/activitystreams",
		"id":        deleteID,
		"type":      "Delete",
		"actor":     actorURI,
		"published": time.Now().Format(time.RFC3339),
		"to": []string{
			"https://www.w3.org/ns/activitystreams#Public",
		},
		"cc": []string{
			fmt.Sprintf("https://%s/users/%s/followers", conf.Conf.SslDomain, localAccount.Username),
		},
		"object": noteURI,
	}

	// Collect inboxes to deliver to
	inboxes := make(map[string]bool)

	// Get all followers
	err, followers := database.ReadFollowersByAccountId(localAccount.Id)
	if err != nil {
		log.Printf("Outbox: Failed to get followers for Delete: %v", err)
	} else if followers != nil {
		for _, follower := range *followers {
			// Skip local followers - they don't need federation delivery
			if follower.IsLocal {
				continue
			}
			err, remoteActor := database.ReadRemoteAccountById(follower.AccountId)
			if err != nil {
				log.Printf("Outbox: Failed to get remote actor %s: %v", follower.AccountId, err)
				continue
			}
			inboxes[remoteActor.InboxURI] = true
		}
	}

	// Get active relays and add their inboxes
	err, relays := database.ReadActiveRelays()
	if err == nil && relays != nil {
		for _, relay := range *relays {
			inboxes[relay.InboxURI] = true
			log.Printf("Outbox: Will also deliver Delete to relay %s", relay.ActorURI)
		}
	}

	if len(inboxes) == 0 {
		log.Printf("Outbox: No inboxes to deliver Delete to")
		return nil
	}

	// Queue delivery to each unique inbox
	for inboxURI := range inboxes {
		queueItem := &domain.DeliveryQueueItem{
			Id:           uuid.New(),
			InboxURI:     inboxURI,
			ActivityJSON: mustMarshal(deleteActivity),
			Attempts:     0,
			NextRetryAt:  time.Now(),
			CreatedAt:    time.Now(),
		}

		if err := database.EnqueueDelivery(queueItem); err != nil {
			log.Printf("Outbox: Failed to queue Delete delivery to %s: %v", inboxURI, err)
		}
	}

	log.Printf("Outbox: Queued Delete activity for note %s to %d inboxes", noteId, len(inboxes))
	return nil
}

// SendFollow sends a Follow activity to a remote actor.
// This is the production wrapper that uses the default HTTP client and database.
func SendFollow(localAccount *domain.Account, remoteActorURI string, conf *util.AppConfig) error {
	return SendFollowWithDeps(localAccount, remoteActorURI, conf, defaultHTTPClient, NewDBWrapper())
}

// SendFollowWithDeps sends a Follow activity to a remote actor.
// This version accepts dependencies for testing.
func SendFollowWithDeps(localAccount *domain.Account, remoteActorURI string, conf *util.AppConfig, client HTTPClient, database Database) error {
	// Fetch remote actor
	remoteActor, err := GetOrFetchActorWithDeps(remoteActorURI, client, database)
	if err != nil {
		return fmt.Errorf("failed to fetch remote actor: %w", err)
	}

	// Check if trying to follow yourself
	if remoteActor.Domain == conf.Conf.SslDomain && remoteActor.Username == localAccount.Username {
		log.Printf("SendFollow: User %s attempted to follow themselves", localAccount.Username)
		return fmt.Errorf("self-follow not allowed on stegodon for now")
	}

	// Check if already following this user
	err, existingFollow := database.ReadFollowByAccountIds(localAccount.Id, remoteActor.Id)
	if err != sql.ErrNoRows && err != nil {
		// Database error (not "not found")
		log.Printf("SendFollow: Error checking existing follow: %v", err)
		return fmt.Errorf("failed to check existing follow: %w", err)
	}
	if existingFollow != nil {
		// Follow relationship already exists - check if accepted
		if existingFollow.Accepted {
			// Already following and accepted
			log.Printf("SendFollow: User %s is already following %s@%s (accepted)", localAccount.Username, remoteActor.Username, remoteActor.Domain)
			return fmt.Errorf("already following %s@%s", remoteActor.Username, remoteActor.Domain)
		} else {
			// Follow exists but pending acceptance
			log.Printf("SendFollow: User %s has pending follow request to %s@%s", localAccount.Username, remoteActor.Username, remoteActor.Domain)
			return fmt.Errorf("follow pending %s@%s", remoteActor.Username, remoteActor.Domain)
		}
	}

	// Not following yet, create the follow
	followID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	follow := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       followID,
		"type":     "Follow",
		"actor":    actorURI,
		"object":   remoteActorURI,
	}

	// Store follow relationship as pending
	followRecord := &domain.Follow{
		Id:              uuid.New(),
		AccountId:       localAccount.Id,
		TargetAccountId: remoteActor.Id,
		URI:             followID,
		Accepted:        false, // Pending until Accept received
		CreatedAt:       time.Now(),
	}

	if err := database.CreateFollow(followRecord); err != nil {
		return fmt.Errorf("failed to store follow: %w", err)
	}

	// Send Follow activity
	return SendActivityWithDeps(follow, remoteActor.InboxURI, localAccount, conf, client)
}

// SendUndo sends an Undo activity for a Follow (i.e., unfollow).
// This is the production wrapper that uses the default HTTP client.
func SendUndo(localAccount *domain.Account, follow *domain.Follow, remoteActor *domain.RemoteAccount, conf *util.AppConfig) error {
	return SendUndoWithDeps(localAccount, follow, remoteActor, conf, defaultHTTPClient)
}

// SendUndoWithDeps sends an Undo activity for a Follow (i.e., unfollow).
// This version accepts dependencies for testing.
func SendUndoWithDeps(localAccount *domain.Account, follow *domain.Follow, remoteActor *domain.RemoteAccount, conf *util.AppConfig, client HTTPClient) error {
	undoID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	// Create Undo activity with embedded Follow object
	undo := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       undoID,
		"type":     "Undo",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     follow.URI,
			"type":   "Follow",
			"actor":  actorURI,
			"object": remoteActor.ActorURI,
		},
	}

	log.Printf("Outbox: Sending Undo (unfollow) from %s to %s@%s", localAccount.Username, remoteActor.Username, remoteActor.Domain)
	return SendActivityWithDeps(undo, remoteActor.InboxURI, localAccount, conf, client)
}

// SendLike sends a Like activity for a note.
// This is the production wrapper that uses the default HTTP client and database.
func SendLike(localAccount *domain.Account, noteURI string, conf *util.AppConfig) error {
	return SendLikeWithDeps(localAccount, noteURI, conf, defaultHTTPClient, NewDBWrapper())
}

// SendLikeWithDeps sends a Like activity for a note.
// This version accepts dependencies for testing.
func SendLikeWithDeps(localAccount *domain.Account, noteURI string, conf *util.AppConfig, client HTTPClient, database Database) error {
	// Find the author of the note to deliver the Like
	authorURI := extractAuthorFromURI(noteURI, database, conf)
	if authorURI == "" {
		return fmt.Errorf("could not determine note author for %s", noteURI)
	}

	// Check if this is a local note (don't send ActivityPub for local likes)
	if strings.Contains(authorURI, conf.Conf.SslDomain) {
		log.Printf("Outbox: Skipping Like delivery for local note %s", noteURI)
		return nil
	}

	// Fetch remote actor to get inbox
	remoteActor, err := GetOrFetchActorWithDeps(authorURI, client, database)
	if err != nil {
		return fmt.Errorf("failed to fetch note author: %w", err)
	}

	likeID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	like := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       likeID,
		"type":     "Like",
		"actor":    actorURI,
		"object":   noteURI,
	}

	log.Printf("Outbox: Sending Like from %s for note %s to %s@%s", localAccount.Username, noteURI, remoteActor.Username, remoteActor.Domain)
	return SendActivityWithDeps(like, remoteActor.InboxURI, localAccount, conf, client)
}

// SendUndoLike sends an Undo activity for a Like (i.e., unlike).
// This is the production wrapper that uses the default HTTP client and database.
func SendUndoLike(localAccount *domain.Account, noteURI string, likeURI string, conf *util.AppConfig) error {
	return SendUndoLikeWithDeps(localAccount, noteURI, likeURI, conf, defaultHTTPClient, NewDBWrapper())
}

// SendUndoLikeWithDeps sends an Undo activity for a Like (i.e., unlike).
// This version accepts dependencies for testing.
func SendUndoLikeWithDeps(localAccount *domain.Account, noteURI string, likeURI string, conf *util.AppConfig, client HTTPClient, database Database) error {
	// Find the author of the note to deliver the Undo
	authorURI := extractAuthorFromURI(noteURI, database, conf)
	if authorURI == "" {
		return fmt.Errorf("could not determine note author for %s", noteURI)
	}

	// Check if this is a local note (don't send ActivityPub for local unlikes)
	if strings.Contains(authorURI, conf.Conf.SslDomain) {
		log.Printf("Outbox: Skipping Undo Like delivery for local note %s", noteURI)
		return nil
	}

	// Fetch remote actor to get inbox
	remoteActor, err := GetOrFetchActorWithDeps(authorURI, client, database)
	if err != nil {
		return fmt.Errorf("failed to fetch note author: %w", err)
	}

	undoID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	undo := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       undoID,
		"type":     "Undo",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     likeURI,
			"type":   "Like",
			"actor":  actorURI,
			"object": noteURI,
		},
	}

	log.Printf("Outbox: Sending Undo Like from %s for note %s to %s@%s", localAccount.Username, noteURI, remoteActor.Username, remoteActor.Domain)
	return SendActivityWithDeps(undo, remoteActor.InboxURI, localAccount, conf, client)
}

// SendRelayFollow subscribes to a relay by sending a Follow activity.
// This is the production wrapper that uses the default HTTP client and database.
func SendRelayFollow(localAccount *domain.Account, relayActorURI string, conf *util.AppConfig) error {
	return SendRelayFollowWithDeps(localAccount, relayActorURI, conf, defaultHTTPClient, NewDBWrapper())
}

// SendRelayFollowWithDeps subscribes to a relay by sending a Follow activity.
// This version accepts dependencies for testing.
func SendRelayFollowWithDeps(localAccount *domain.Account, relayActorURI string, conf *util.AppConfig, client HTTPClient, database Database) error {
	// Fetch the relay actor to get inbox and validate it's a relay
	relayActor, err := FetchRemoteActorWithDeps(relayActorURI, client, database)
	if err != nil {
		return fmt.Errorf("failed to fetch relay actor: %w", err)
	}

	// Check if already subscribed
	err, existingRelay := database.ReadRelayByActorURI(relayActorURI)
	if err == nil && existingRelay != nil {
		if existingRelay.Status == "active" {
			return fmt.Errorf("already subscribed to relay %s", relayActorURI)
		}
		if existingRelay.Status == "pending" {
			return fmt.Errorf("subscription to relay %s is pending", relayActorURI)
		}
		// If status is "failed", we allow retry by deleting and recreating
		if err := database.DeleteRelay(existingRelay.Id); err != nil {
			log.Printf("Outbox: Failed to delete old relay record: %v", err)
		}
	}

	// Create follow activity
	followID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	// Use the public address as the object for relay follows
	// This is compatible with both FediBuzz and YUKIMOCHI Activity-Relay
	// YUKIMOCHI requires either object=Public or actor path ending in /relay
	follow := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       followID,
		"type":     "Follow",
		"actor":    actorURI,
		"object":   "https://www.w3.org/ns/activitystreams#Public",
	}

	// Store relay record as pending (include follow URI for later Undo)
	relay := &domain.Relay{
		Id:        uuid.New(),
		ActorURI:  relayActorURI,
		InboxURI:  relayActor.InboxURI,
		FollowURI: followID,
		Name:      relayActor.DisplayName,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	if err := database.CreateRelay(relay); err != nil {
		return fmt.Errorf("failed to store relay: %w", err)
	}

	// Send Follow activity to relay
	log.Printf("Outbox: Sending Follow to relay %s from %s", relayActorURI, localAccount.Username)
	return SendActivityWithDeps(follow, relayActor.InboxURI, localAccount, conf, client)
}

// SendRelayUnfollow unsubscribes from a relay by sending an Undo Follow activity.
// This is the production wrapper that uses the default HTTP client.
func SendRelayUnfollow(localAccount *domain.Account, relay *domain.Relay, conf *util.AppConfig) error {
	return SendRelayUnfollowWithDeps(localAccount, relay, conf, defaultHTTPClient)
}

// SendRelayUnfollowWithDeps unsubscribes from a relay by sending an Undo Follow activity.
// This version accepts dependencies for testing.
func SendRelayUnfollowWithDeps(localAccount *domain.Account, relay *domain.Relay, conf *util.AppConfig, client HTTPClient) error {
	undoID := fmt.Sprintf("https://%s/activities/%s", conf.Conf.SslDomain, uuid.New().String())
	actorURI := fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localAccount.Username)

	// Use the stored Follow URI if available, otherwise construct one
	followID := relay.FollowURI
	if followID == "" {
		// Fallback for relays created before we stored the follow URI
		followID = fmt.Sprintf("https://%s/relay-follows/%s", conf.Conf.SslDomain, relay.Id.String())
	}

	undo := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       undoID,
		"type":     "Undo",
		"actor":    actorURI,
		"object": map[string]any{
			"id":     followID,
			"type":   "Follow",
			"actor":  actorURI,
			"object": "https://www.w3.org/ns/activitystreams#Public",
		},
	}

	log.Printf("Outbox: Sending Undo Follow (unsubscribe) to relay %s from %s", relay.ActorURI, localAccount.Username)
	return SendActivityWithDeps(undo, relay.InboxURI, localAccount, conf, client)
}

// mustMarshal marshals v to JSON, panicking on error
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return string(b)
}

// extractAuthorFromURI attempts to extract the author URI from a note/activity URI
// This is used to add the parent author to cc when creating a reply
func extractAuthorFromURI(objectURI string, database Database, conf *util.AppConfig) string {
	// First, check if we have a stored activity with this object
	err, activity := database.ReadActivityByObjectURI(objectURI)
	if err == nil && activity != nil {
		return activity.ActorURI
	}

	// Try to check if it's a local note
	err, localNote := database.ReadNoteByURI(objectURI)
	if err == nil && localNote != nil {
		// It's a local note - return the local author's actor URI
		// This ensures replies to local users are delivered to their inbox
		return fmt.Sprintf("https://%s/users/%s", conf.Conf.SslDomain, localNote.CreatedBy)
	}

	// Can't determine author - caller should handle gracefully
	log.Printf("extractAuthorFromURI: Could not determine author for %s", objectURI)
	return ""
}

// resolveMentionURI resolves a @username@domain mention to an ActivityPub actor URI
// using WebFinger lookup
func resolveMentionURI(username, domain string) (string, error) {
	webfingerURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s",
		domain, username, domain)

	req, err := http.NewRequest("GET", webfingerURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/jrd+json")
	req.Header.Set("User-Agent", "stegodon/1.0 ActivityPub")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("webfinger request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("webfinger failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// WebFingerResponse structure for parsing
	type webFingerLink struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	}
	type webFingerResponse struct {
		Subject string          `json:"subject"`
		Links   []webFingerLink `json:"links"`
	}

	var result webFingerResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse webfinger response: %w", err)
	}

	// Find self link with ActivityPub-compatible type
	for _, link := range result.Links {
		if link.Rel == "self" {
			if link.Type == "application/activity+json" ||
				link.Type == "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\"" {
				return link.Href, nil
			}
		}
	}

	return "", fmt.Errorf("no ActivityPub actor found in webfinger response")
}

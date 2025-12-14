package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// SQL for new ActivityPub tables
const (
	// Follow relationships table
	sqlCreateFollowsTable = `CREATE TABLE IF NOT EXISTS follows (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		target_account_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted INTEGER DEFAULT 0
	)`

	sqlCreateFollowsIndices = `
		CREATE INDEX IF NOT EXISTS idx_follows_account_id ON follows(account_id);
		CREATE INDEX IF NOT EXISTS idx_follows_target_account_id ON follows(target_account_id);
		CREATE INDEX IF NOT EXISTS idx_follows_uri ON follows(uri);
	`

	// Remote accounts cache table
	sqlCreateRemoteAccountsTable = `CREATE TABLE IF NOT EXISTS remote_accounts (
		id TEXT NOT NULL PRIMARY KEY,
		username TEXT NOT NULL,
		domain TEXT NOT NULL,
		actor_uri TEXT UNIQUE NOT NULL,
		display_name TEXT,
		summary TEXT,
		inbox_uri TEXT NOT NULL,
		outbox_uri TEXT,
		public_key_pem TEXT NOT NULL,
		avatar_url TEXT,
		last_fetched_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(username, domain)
	)`

	sqlCreateRemoteAccountsIndices = `
		CREATE INDEX IF NOT EXISTS idx_remote_accounts_actor_uri ON remote_accounts(actor_uri);
		CREATE INDEX IF NOT EXISTS idx_remote_accounts_domain ON remote_accounts(domain);
	`

	// Activities log table (for deduplication & debugging)
	sqlCreateActivitiesTable = `CREATE TABLE IF NOT EXISTS activities (
		id TEXT NOT NULL PRIMARY KEY,
		activity_uri TEXT UNIQUE NOT NULL,
		activity_type TEXT NOT NULL,
		actor_uri TEXT NOT NULL,
		object_uri TEXT,
		raw_json TEXT NOT NULL,
		processed INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		local INTEGER DEFAULT 0
	)`

	sqlCreateActivitiesIndices = `
		CREATE INDEX IF NOT EXISTS idx_activities_uri ON activities(activity_uri);
		CREATE INDEX IF NOT EXISTS idx_activities_processed ON activities(processed);
		CREATE INDEX IF NOT EXISTS idx_activities_type ON activities(activity_type);
		CREATE INDEX IF NOT EXISTS idx_activities_created_at ON activities(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_activities_object_uri ON activities(object_uri);
		CREATE INDEX IF NOT EXISTS idx_activities_from_relay ON activities(from_relay);
	`

	// Likes/favorites table
	sqlCreateLikesTable = `CREATE TABLE IF NOT EXISTS likes (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		note_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, note_id)
	)`

	sqlCreateLikesIndices = `
		CREATE INDEX IF NOT EXISTS idx_likes_note_id ON likes(note_id);
		CREATE INDEX IF NOT EXISTS idx_likes_account_id ON likes(account_id);
		CREATE INDEX IF NOT EXISTS idx_likes_object_uri ON likes(object_uri);
	`

	// Boosts/announces table
	sqlCreateBoostsTable = `CREATE TABLE IF NOT EXISTS boosts (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		note_id TEXT NOT NULL,
		uri TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(account_id, note_id)
	)`

	sqlCreateBoostsIndices = `
		CREATE INDEX IF NOT EXISTS idx_boosts_note_id ON boosts(note_id);
		CREATE INDEX IF NOT EXISTS idx_boosts_account_id ON boosts(account_id);
	`

	// Delivery queue table
	sqlCreateDeliveryQueueTable = `CREATE TABLE IF NOT EXISTS delivery_queue (
		id TEXT NOT NULL PRIMARY KEY,
		inbox_uri TEXT NOT NULL,
		activity_json TEXT NOT NULL,
		attempts INTEGER DEFAULT 0,
		next_retry_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateDeliveryQueueIndices = `
		CREATE INDEX IF NOT EXISTS idx_delivery_queue_next_retry ON delivery_queue(next_retry_at);
	`

	// Hashtags table
	sqlCreateHashtagsTable = `CREATE TABLE IF NOT EXISTS hashtags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		usage_count INTEGER DEFAULT 0,
		last_used_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`

	sqlCreateHashtagsIndices = `
		CREATE INDEX IF NOT EXISTS idx_hashtags_name ON hashtags(name);
		CREATE INDEX IF NOT EXISTS idx_hashtags_usage ON hashtags(usage_count DESC);
	`

	// Note-hashtag relationship table
	sqlCreateNoteHashtagsTable = `CREATE TABLE IF NOT EXISTS note_hashtags (
		note_id TEXT NOT NULL,
		hashtag_id INTEGER NOT NULL,
		PRIMARY KEY (note_id, hashtag_id),
		FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
		FOREIGN KEY (hashtag_id) REFERENCES hashtags(id) ON DELETE CASCADE
	)`

	sqlCreateNoteHashtagsIndices = `
		CREATE INDEX IF NOT EXISTS idx_note_hashtags_note_id ON note_hashtags(note_id);
		CREATE INDEX IF NOT EXISTS idx_note_hashtags_hashtag_id ON note_hashtags(hashtag_id);
	`

	// Note-mention relationship table (stores @user@domain mentions in notes)
	sqlCreateNoteMentionsTable = `CREATE TABLE IF NOT EXISTS note_mentions (
		id TEXT PRIMARY KEY,
		note_id TEXT NOT NULL,
		mentioned_actor_uri TEXT NOT NULL,
		mentioned_username TEXT NOT NULL,
		mentioned_domain TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
	)`

	sqlCreateNoteMentionsIndices = `
		CREATE INDEX IF NOT EXISTS idx_note_mentions_note_id ON note_mentions(note_id);
		CREATE INDEX IF NOT EXISTS idx_note_mentions_actor_uri ON note_mentions(mentioned_actor_uri);
	`

	// Relays table for ActivityPub relay subscriptions
	sqlCreateRelaysTable = `CREATE TABLE IF NOT EXISTS relays (
		id TEXT NOT NULL PRIMARY KEY,
		actor_uri TEXT UNIQUE NOT NULL,
		inbox_uri TEXT NOT NULL,
		follow_uri TEXT,
		name TEXT,
		status TEXT DEFAULT 'pending',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		accepted_at TIMESTAMP
	)`

	sqlCreateRelaysIndices = `
		CREATE INDEX IF NOT EXISTS idx_relays_status ON relays(status);
	`

	// Notifications table for user notifications
	sqlCreateNotificationsTable = `CREATE TABLE IF NOT EXISTS notifications (
		id TEXT NOT NULL PRIMARY KEY,
		account_id TEXT NOT NULL,
		notification_type TEXT NOT NULL,
		actor_id TEXT,
		actor_username TEXT,
		actor_domain TEXT,
		note_id TEXT,
		note_uri TEXT,
		note_preview TEXT,
		read INTEGER DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (account_id) REFERENCES accounts(id) ON DELETE CASCADE
	)`

	sqlCreateNotificationsIndices = `
		CREATE INDEX IF NOT EXISTS idx_notifications_account_id ON notifications(account_id);
		CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_notifications_account_read ON notifications(account_id, read);
	`

	// Extend existing tables with new columns
	sqlExtendAccountsTable = `
		ALTER TABLE accounts ADD COLUMN display_name TEXT;
		ALTER TABLE accounts ADD COLUMN summary TEXT;
		ALTER TABLE accounts ADD COLUMN avatar_url TEXT;
	`

	sqlExtendNotesTable = `
		ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public';
		ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT;
		ALTER TABLE notes ADD COLUMN object_uri TEXT;
		ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1;
		ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0;
		ALTER TABLE notes ADD COLUMN content_warning TEXT;
	`

	sqlCreateNotesIndices = `
		CREATE INDEX IF NOT EXISTS idx_notes_user_id ON notes(user_id);
		CREATE INDEX IF NOT EXISTS idx_notes_created_at ON notes(created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_notes_object_uri ON notes(object_uri);
		CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri ON notes(in_reply_to_uri);
	`
)

// RunMigrations executes all database migrations
func (db *DB) RunMigrations() error {
	return db.wrapTransaction(func(tx *sql.Tx) error {
		// Create new tables
		if err := db.createTableIfNotExists(tx, sqlCreateFollowsTable, "follows"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateRemoteAccountsTable, "remote_accounts"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateActivitiesTable, "activities"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateLikesTable, "likes"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateBoostsTable, "boosts"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateDeliveryQueueTable, "delivery_queue"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateHashtagsTable, "hashtags"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNoteHashtagsTable, "note_hashtags"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNoteMentionsTable, "note_mentions"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateRelaysTable, "relays"); err != nil {
			return err
		}
		if err := db.createTableIfNotExists(tx, sqlCreateNotificationsTable, "notifications"); err != nil {
			return err
		}

		// Create indices
		if _, err := tx.Exec(sqlCreateFollowsIndices); err != nil {
			log.Printf("Warning: Failed to create follows indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateRemoteAccountsIndices); err != nil {
			log.Printf("Warning: Failed to create remote_accounts indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateActivitiesIndices); err != nil {
			log.Printf("Warning: Failed to create activities indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateLikesIndices); err != nil {
			log.Printf("Warning: Failed to create likes indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateBoostsIndices); err != nil {
			log.Printf("Warning: Failed to create boosts indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateDeliveryQueueIndices); err != nil {
			log.Printf("Warning: Failed to create delivery_queue indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateHashtagsIndices); err != nil {
			log.Printf("Warning: Failed to create hashtags indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNoteHashtagsIndices); err != nil {
			log.Printf("Warning: Failed to create note_hashtags indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNoteMentionsIndices); err != nil {
			log.Printf("Warning: Failed to create note_mentions indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateRelaysIndices); err != nil {
			log.Printf("Warning: Failed to create relays indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNotificationsIndices); err != nil {
			log.Printf("Warning: Failed to create notifications indices: %v", err)
		}
		if _, err := tx.Exec(sqlCreateNotesIndices); err != nil {
			log.Printf("Warning: Failed to create notes indices: %v", err)
		}

		// Extend existing tables (ignore errors if columns already exist)
		db.extendExistingTables(tx)

		// Backfill object_uri for existing activities
		if err := db.backfillActivityObjectURIs(tx); err != nil {
			log.Printf("Warning: Failed to backfill activity object_uri: %v", err)
		}

		// Add username uniqueness constraint (handles duplicates gracefully)
		if err := db.addUsernameUniqueConstraint(tx); err != nil {
			log.Printf("Warning: Failed to add username unique constraint: %v", err)
		}

		// Backfill reply counts for existing notes and activities
		if err := db.backfillReplyCounts(tx); err != nil {
			log.Printf("Warning: Failed to backfill reply counts: %v", err)
		}

		// Fix orphaned Update activities (convert to Create so they show in timeline)
		if err := db.fixOrphanedUpdateActivities(tx); err != nil {
			log.Printf("Warning: Failed to fix orphaned Update activities: %v", err)
		}

		return nil
	})
}

func (db *DB) createTableIfNotExists(tx *sql.Tx, createSQL string, tableName string) error {
	_, err := tx.Exec(createSQL)
	if err != nil {
		log.Printf("Error creating table %s: %v", tableName, err)
		return err
	}
	log.Printf("Table %s created or already exists", tableName)
	return nil
}

func (db *DB) extendExistingTables(tx *sql.Tx) {
	// Try to add columns to accounts table (ignore errors if they exist)
	tx.Exec("ALTER TABLE accounts ADD COLUMN display_name TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN summary TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN avatar_url TEXT")
	tx.Exec("ALTER TABLE accounts ADD COLUMN is_admin INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE accounts ADD COLUMN muted INTEGER DEFAULT 0")

	// Try to add columns to notes table (ignore errors if they exist)
	tx.Exec("ALTER TABLE notes ADD COLUMN visibility TEXT DEFAULT 'public'")
	tx.Exec("ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN object_uri TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN federated INTEGER DEFAULT 1")
	tx.Exec("ALTER TABLE notes ADD COLUMN sensitive INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN content_warning TEXT")
	tx.Exec("ALTER TABLE notes ADD COLUMN edited_at TIMESTAMP")

	// Engagement count columns for notes (denormalized for performance)
	tx.Exec("ALTER TABLE notes ADD COLUMN reply_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN like_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE notes ADD COLUMN boost_count INTEGER DEFAULT 0")

	// Engagement count columns for activities (remote posts)
	tx.Exec("ALTER TABLE activities ADD COLUMN reply_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE activities ADD COLUMN like_count INTEGER DEFAULT 0")
	tx.Exec("ALTER TABLE activities ADD COLUMN boost_count INTEGER DEFAULT 0")

	// Add is_local column to follows table to support local follows
	tx.Exec("ALTER TABLE follows ADD COLUMN is_local INTEGER DEFAULT 0")

	// Add account_id column to delivery_queue table to support account-based cleanup
	tx.Exec("ALTER TABLE delivery_queue ADD COLUMN account_id TEXT")

	// Add object_uri column to likes table for remote post likes
	tx.Exec("ALTER TABLE likes ADD COLUMN object_uri TEXT")

	// Add unique index for remote post likes (account_id + object_uri)
	// This allows one like per account per remote post (identified by object_uri)
	tx.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_likes_account_object_uri ON likes(account_id, object_uri) WHERE object_uri IS NOT NULL AND object_uri != ''")

	// Add follow_uri column to relays table for proper Undo Follow
	tx.Exec("ALTER TABLE relays ADD COLUMN follow_uri TEXT")

	// Add paused column to relays table for pause/resume functionality
	tx.Exec("ALTER TABLE relays ADD COLUMN paused INTEGER DEFAULT 0")

	// Add from_relay column to activities table to track relay-forwarded content
	tx.Exec("ALTER TABLE activities ADD COLUMN from_relay INTEGER DEFAULT 0")

	log.Println("Extended existing tables with new columns")
}

// backfillActivityObjectURIs extracts object_uri from raw_json for activities that are missing it
func (db *DB) backfillActivityObjectURIs(tx *sql.Tx) error {
	// Find activities with empty object_uri
	rows, err := tx.Query(`SELECT id, raw_json FROM activities WHERE object_uri IS NULL OR object_uri = ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	updated := 0
	for rows.Next() {
		var id, rawJSON string
		if err := rows.Scan(&id, &rawJSON); err != nil {
			log.Printf("Warning: Failed to scan activity: %v", err)
			continue
		}

		// Parse the raw JSON to extract object ID
		var activity struct {
			Object any `json:"object"`
		}
		if err := json.Unmarshal([]byte(rawJSON), &activity); err != nil {
			log.Printf("Warning: Failed to parse activity JSON for ID %s: %v", id, err)
			continue
		}

		// Extract object URI
		var objectURI string
		if activity.Object != nil {
			switch obj := activity.Object.(type) {
			case string:
				objectURI = obj
			case map[string]any:
				if idVal, ok := obj["id"].(string); ok {
					objectURI = idVal
				}
			}
		}

		// Update the activity if we found an object URI
		if objectURI != "" {
			_, err := tx.Exec(`UPDATE activities SET object_uri = ? WHERE id = ?`, objectURI, id)
			if err != nil {
				log.Printf("Warning: Failed to update activity %s: %v", id, err)
			} else {
				updated++
			}
		}
	}

	if updated > 0 {
		log.Printf("Backfilled object_uri for %d activities", updated)
	}

	return nil
}

// addUsernameUniqueConstraint renames duplicate usernames and adds UNIQUE constraint
func (db *DB) addUsernameUniqueConstraint(tx *sql.Tx) error {
	// Find duplicate usernames (case-insensitive)
	rows, err := tx.Query(`
		SELECT username, COUNT(*) as count
		FROM accounts
		GROUP BY LOWER(username)
		HAVING count > 1
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect duplicate usernames
	var duplicates []string
	for rows.Next() {
		var username string
		var count int
		if err := rows.Scan(&username, &count); err != nil {
			log.Printf("Warning: Failed to scan duplicate username: %v", err)
			continue
		}
		duplicates = append(duplicates, username)
	}

	// Process each duplicate username
	for _, username := range duplicates {
		// Get all accounts with this username, ordered by creation time
		accountRows, err := tx.Query(`
			SELECT id, username, created_at
			FROM accounts
			WHERE LOWER(username) = LOWER(?)
			ORDER BY created_at ASC
		`, username)
		if err != nil {
			log.Printf("Warning: Failed to query accounts for username '%s': %v", username, err)
			continue
		}

		var accounts []struct {
			id        string
			username  string
			createdAt string
		}

		for accountRows.Next() {
			var acc struct {
				id        string
				username  string
				createdAt string
			}
			if err := accountRows.Scan(&acc.id, &acc.username, &acc.createdAt); err != nil {
				log.Printf("Warning: Failed to scan account: %v", err)
				continue
			}
			accounts = append(accounts, acc)
		}
		accountRows.Close()

		// Keep the first (oldest) account, rename the rest
		for i := 1; i < len(accounts); i++ {
			newUsername := accounts[i].username + "_" + fmt.Sprintf("%d", i+1)

			// Ensure new username doesn't exceed any length limits and is valid
			if len(newUsername) > 50 {
				newUsername = accounts[i].username[:45] + "_" + fmt.Sprintf("%d", i+1)
			}

			_, err := tx.Exec(`UPDATE accounts SET username = ? WHERE id = ?`, newUsername, accounts[i].id)
			if err != nil {
				log.Printf("Warning: Failed to rename duplicate username '%s' (id: %s) to '%s': %v",
					accounts[i].username, accounts[i].id, newUsername, err)
			} else {
				log.Printf("Renamed duplicate username '%s' (id: %s, created: %s) to '%s'",
					accounts[i].username, accounts[i].id, accounts[i].createdAt, newUsername)
			}
		}
	}

	// Add UNIQUE constraint (case-insensitive)
	_, err = tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_username ON accounts(username COLLATE NOCASE)`)
	if err != nil {
		return fmt.Errorf("failed to create unique index on username: %v", err)
	}

	log.Println("Added UNIQUE constraint to accounts.username column")
	return nil
}

// backfillReplyCounts recalculates reply_count for all notes and activities
// This runs once during migration to populate the denormalized counts
// It uses recursive counting to get the total of all nested replies
func (db *DB) backfillReplyCounts(tx *sql.Tx) error {
	// Check if we've already backfilled (if any reply_count > 0, skip)
	var hasData int
	err := tx.QueryRow(`SELECT COUNT(*) FROM notes WHERE reply_count > 0`).Scan(&hasData)
	if err == nil && hasData > 0 {
		log.Println("Reply counts already backfilled, skipping")
		return nil
	}

	log.Println("Backfilling reply counts for notes and activities (recursive)...")

	// Reset all counts to 0
	tx.Exec(`UPDATE notes SET reply_count = 0`)
	tx.Exec(`UPDATE activities SET reply_count = 0`)

	// Get all notes with in_reply_to_uri (these are replies)
	rows, err := tx.Query(`SELECT in_reply_to_uri FROM notes WHERE in_reply_to_uri IS NOT NULL AND in_reply_to_uri != ''`)
	if err != nil {
		log.Printf("Warning: Failed to query note replies: %v", err)
	} else {
		defer rows.Close()
		for rows.Next() {
			var inReplyTo string
			if err := rows.Scan(&inReplyTo); err == nil && inReplyTo != "" {
				// Increment all ancestors
				db.incrementReplyCountRecursive(tx, inReplyTo, make(map[string]bool))
			}
		}
	}

	// Get all Create activities with inReplyTo (remote replies)
	// Skip activities that are duplicates of local notes:
	// 1. Same object_uri exists in notes table, OR
	// 2. Activity object_uri contains a local note ID pattern (/notes/{uuid})
	rows2, err := tx.Query(`
		SELECT a.object_uri, a.raw_json
		FROM activities a
		WHERE a.activity_type = 'Create'
		AND a.raw_json LIKE '%"inReplyTo":"http%'
		AND NOT EXISTS (
			SELECT 1 FROM notes n
			WHERE (n.object_uri = a.object_uri AND n.object_uri IS NOT NULL AND n.object_uri != '')
			   OR (a.object_uri LIKE '%/notes/' || n.id || '%')
		)
	`)
	if err != nil {
		log.Printf("Warning: Failed to query activity replies: %v", err)
	} else {
		defer rows2.Close()
		for rows2.Next() {
			var objectURI, rawJSON string
			if err := rows2.Scan(&objectURI, &rawJSON); err == nil {
				inReplyTo := extractInReplyToFromJSON(rawJSON)
				if inReplyTo != "" {
					// Increment all ancestors
					db.incrementReplyCountRecursive(tx, inReplyTo, make(map[string]bool))
				}
			}
		}
	}

	log.Println("Completed backfilling reply counts")
	return nil
}

// fixOrphanedUpdateActivities converts Update activities that have no corresponding Create
// to Create activities so they show up in the timeline.
// This happens when we followed a user after their original post, and only received the Update.
func (db *DB) fixOrphanedUpdateActivities(tx *sql.Tx) error {
	// Find Update activities for Notes where we don't have a Create activity for the same object_uri
	// Group by object_uri and pick the first Update (oldest) to convert to Create
	rows, err := tx.Query(`
		SELECT u.id, u.activity_uri, u.actor_uri, u.object_uri, u.raw_json, u.created_at
		FROM activities u
		WHERE u.activity_type = 'Update'
		AND u.object_uri IS NOT NULL
		AND u.object_uri != ''
		AND NOT EXISTS (
			SELECT 1 FROM activities c
			WHERE c.object_uri = u.object_uri
			AND c.activity_type = 'Create'
		)
		AND u.id = (
			SELECT MIN(u2.id) FROM activities u2
			WHERE u2.object_uri = u.object_uri
			AND u2.activity_type = 'Update'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to query orphaned Update activities: %w", err)
	}
	defer rows.Close()

	converted := 0
	for rows.Next() {
		var id, activityURI, actorURI, objectURI, rawJSON, createdAt string
		if err := rows.Scan(&id, &activityURI, &actorURI, &objectURI, &rawJSON, &createdAt); err != nil {
			log.Printf("Warning: Failed to scan Update activity: %v", err)
			continue
		}

		// Update the activity_type to 'Create' so it shows in timeline
		_, err := tx.Exec(`UPDATE activities SET activity_type = 'Create' WHERE id = ?`, id)
		if err != nil {
			log.Printf("Warning: Failed to convert Update %s to Create: %v", id, err)
		} else {
			converted++
			log.Printf("Converted orphaned Update to Create: %s (object: %s)", activityURI, objectURI)
		}
	}

	if converted > 0 {
		log.Printf("Converted %d orphaned Update activities to Create", converted)
	}

	return nil
}

// MigratePerformanceIndexes adds performance-critical indexes that were missing
// These indexes speed up threading queries and relay content filtering
func (db *DB) MigratePerformanceIndexes() error {
	log.Println("Checking for missing performance indexes...")

	// Add index on notes.in_reply_to_uri for faster threading queries
	_, err := db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_notes_in_reply_to_uri ON notes(in_reply_to_uri)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_notes_in_reply_to_uri: %v", err)
	}

	// Add index on activities.object_uri for faster deduplication checks
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_object_uri ON activities(object_uri)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_activities_object_uri: %v", err)
	}

	// Add index on activities.from_relay for faster relay content filtering
	_, err = db.db.Exec(`CREATE INDEX IF NOT EXISTS idx_activities_from_relay ON activities(from_relay)`)
	if err != nil {
		log.Printf("Warning: Failed to create idx_activities_from_relay: %v", err)
	}

	log.Println("Performance indexes migration complete")
	return nil
}

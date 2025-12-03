package db

import (
	"database/sql"
	"testing"
	"time"

	"github.com/deemkeen/stegodon/domain"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// setupMigrationTestDB creates a database without username UNIQUE constraint
func setupMigrationTestDB(t *testing.T) *DB {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory database: %v", err)
	}

	db := &DB{db: sqlDB}

	// Create accounts table WITHOUT UNIQUE constraint on username
	_, err = db.db.Exec(`CREATE TABLE IF NOT EXISTS accounts(
		id TEXT NOT NULL PRIMARY KEY,
		username TEXT NOT NULL,
		publickey TEXT UNIQUE,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		first_time_login INTEGER DEFAULT 1,
		web_public_key TEXT,
		web_private_key TEXT,
		display_name TEXT,
		summary TEXT,
		avatar_url TEXT,
		is_admin INTEGER DEFAULT 0,
		muted INTEGER DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("Failed to create accounts table: %v", err)
	}

	return db
}

func TestAddUsernameUniqueConstraint_NoDuplicates(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	createTestAccount(t, db, id1, "alice", "hash1", "pubkey1", "privkey1")
	createTestAccount(t, db, id2, "bob", "hash2", "pubkey2", "privkey2")
	createTestAccount(t, db, id3, "charlie", "hash3", "pubkey3", "privkey3")

	err := db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	err1, a1 := db.ReadAccById(id1)
	if err1 != nil {
		t.Fatalf("Failed to read alice: %v", err1)
	}
	if a1.Username != "alice" {
		t.Errorf("Expected alice, got %s", a1.Username)
	}

	err2, a2 := db.ReadAccById(id2)
	if err2 != nil {
		t.Fatalf("Failed to read bob: %v", err2)
	}
	if a2.Username != "bob" {
		t.Errorf("Expected bob, got %s", a2.Username)
	}

	err3, a3 := db.ReadAccById(id3)
	if err3 != nil {
		t.Fatalf("Failed to read charlie: %v", err3)
	}
	if a3.Username != "charlie" {
		t.Errorf("Expected charlie, got %s", a3.Username)
	}

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), "alice", "hash4", domain.FALSE, time.Now(), "pub4", "priv4")
	if err == nil {
		t.Error("Expected unique constraint violation")
	}
}

func TestAddUsernameUniqueConstraint_WithDuplicates(t *testing.T) {
	db := setupMigrationTestDB(t)
	defer db.db.Close()

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	_, err := db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id1.String(), "alice", "hash1", domain.FALSE, time.Now().Add(-3*time.Hour), "pub1", "priv1")
	if err != nil {
		t.Fatalf("Failed to create first account: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id2.String(), "alice", "hash2", domain.FALSE, time.Now().Add(-2*time.Hour), "pub2", "priv2")
	if err != nil {
		t.Fatalf("Failed to create second account: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id3.String(), "alice", "hash3", domain.FALSE, time.Now().Add(-1*time.Hour), "pub3", "priv3")
	if err != nil {
		t.Fatalf("Failed to create third account: %v", err)
	}

	err = db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	err1, acc1 := db.ReadAccById(id1)
	if err1 != nil {
		t.Fatalf("Failed to read id1: %v", err1)
	}
	if acc1.Username != "alice" {
		t.Errorf("Expected 'alice', got '%s'", acc1.Username)
	}

	err2, acc2 := db.ReadAccById(id2)
	if err2 != nil {
		t.Fatalf("Failed to read id2: %v", err2)
	}
	if acc2.Username != "alice_2" {
		t.Errorf("Expected 'alice_2', got '%s'", acc2.Username)
	}

	err3, acc3 := db.ReadAccById(id3)
	if err3 != nil {
		t.Fatalf("Failed to read id3: %v", err3)
	}
	if acc3.Username != "alice_3" {
		t.Errorf("Expected 'alice_3', got '%s'", acc3.Username)
	}

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), "alice", "hash4", domain.FALSE, time.Now(), "pub4", "priv4")
	if err == nil {
		t.Error("Expected unique constraint violation after migration")
	}
}

func TestAddUsernameUniqueConstraint_CaseInsensitive(t *testing.T) {
	db := setupMigrationTestDB(t)
	defer db.db.Close()

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	_, err := db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id1.String(), "Alice", "hash1", domain.FALSE, time.Now().Add(-2*time.Hour), "pub1", "priv1")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id2.String(), "ALICE", "hash2", domain.FALSE, time.Now().Add(-1*time.Hour), "pub2", "priv2")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id3.String(), "alice", "hash3", domain.FALSE, time.Now(), "pub3", "priv3")
	if err != nil {
		t.Fatalf("Failed to create account: %v", err)
	}

	err = db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	err1, acc1 := db.ReadAccById(id1)
	if err1 != nil {
		t.Fatalf("Failed to read id1: %v", err1)
	}
	if acc1.Username != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", acc1.Username)
	}

	err2, acc2 := db.ReadAccById(id2)
	if err2 != nil {
		t.Fatalf("Failed to read id2: %v", err2)
	}
	if acc2.Username != "ALICE_2" {
		t.Errorf("Expected 'ALICE_2', got '%s'", acc2.Username)
	}

	err3, acc3 := db.ReadAccById(id3)
	if err3 != nil {
		t.Fatalf("Failed to read id3: %v", err3)
	}
	if acc3.Username != "alice_3" {
		t.Errorf("Expected 'alice_3', got '%s'", acc3.Username)
	}

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), "aLiCe", "hash4", domain.FALSE, time.Now(), "pub4", "priv4")
	if err == nil {
		t.Error("Expected case-insensitive unique constraint violation")
	}
}

func TestAddUsernameUniqueConstraint_MultipleDuplicateSets(t *testing.T) {
	db := setupMigrationTestDB(t)
	defer db.db.Close()

	alice1 := uuid.New()
	alice2 := uuid.New()
	alice3 := uuid.New()
	bob1 := uuid.New()
	bob2 := uuid.New()
	charlie1 := uuid.New()

	baseTime := time.Now().Add(-10 * time.Hour)

	accounts := []struct {
		id       uuid.UUID
		username string
		offset   time.Duration
	}{
		{alice1, "alice", 0},
		{alice2, "alice", 1 * time.Hour},
		{alice3, "alice", 2 * time.Hour},
		{bob1, "bob", 3 * time.Hour},
		{bob2, "bob", 4 * time.Hour},
		{charlie1, "charlie", 5 * time.Hour},
	}

	for i, acc := range accounts {
		_, err := db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			acc.id.String(), acc.username, "hash_"+acc.id.String(), domain.FALSE, baseTime.Add(acc.offset),
			"pub"+string(rune(i)), "priv"+string(rune(i)))
		if err != nil {
			t.Fatalf("Failed to create account %s: %v", acc.username, err)
		}
	}

	err := db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	err1, a1 := db.ReadAccById(alice1)
	if err1 != nil || a1.Username != "alice" {
		t.Errorf("alice1: expected 'alice', got '%s'", a1.Username)
	}
	err2, a2 := db.ReadAccById(alice2)
	if err2 != nil || a2.Username != "alice_2" {
		t.Errorf("alice2: expected 'alice_2', got '%s'", a2.Username)
	}
	err3, a3 := db.ReadAccById(alice3)
	if err3 != nil || a3.Username != "alice_3" {
		t.Errorf("alice3: expected 'alice_3', got '%s'", a3.Username)
	}

	err4, b1 := db.ReadAccById(bob1)
	if err4 != nil || b1.Username != "bob" {
		t.Errorf("bob1: expected 'bob', got '%s'", b1.Username)
	}
	err5, b2 := db.ReadAccById(bob2)
	if err5 != nil || b2.Username != "bob_2" {
		t.Errorf("bob2: expected 'bob_2', got '%s'", b2.Username)
	}

	err6, c1 := db.ReadAccById(charlie1)
	if err6 != nil || c1.Username != "charlie" {
		t.Errorf("charlie: expected 'charlie', got '%s'", c1.Username)
	}
}

func TestAddUsernameUniqueConstraint_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.db.Close()

	id1 := uuid.New()
	id2 := uuid.New()
	createTestAccount(t, db, id1, "alice", "hash1", "pubkey1", "privkey1")
	createTestAccount(t, db, id2, "bob", "hash2", "pubkey2", "privkey2")

	err := db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("First migration failed: %v", err)
	}

	err = db.wrapTransaction(func(tx *sql.Tx) error {
		return db.addUsernameUniqueConstraint(tx)
	})
	if err != nil {
		t.Fatalf("Second migration failed: %v", err)
	}

	_, err = db.db.Exec(`INSERT INTO accounts (id, username, publickey, first_time_login, created_at, web_public_key, web_private_key)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), "alice", "hash3", domain.FALSE, time.Now(), "pub3", "priv3")
	if err == nil {
		t.Error("Expected unique constraint violation after second migration")
	}
}

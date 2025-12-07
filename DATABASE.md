# Database Schema

Stegodon uses SQLite with WAL mode for data storage. The schema supports both local user management and ActivityPub federation.

## Entity Relationship Diagram

```mermaid
erDiagram
    accounts {
        TEXT id PK
        TEXT username UK
        TEXT publickey UK
        TIMESTAMP created_at
        INTEGER first_time_login
        TEXT web_public_key
        TEXT web_private_key
        TEXT display_name
        TEXT summary
        TEXT avatar_url
        INTEGER is_admin
        INTEGER muted
    }

    notes {
        TEXT id PK
        TEXT user_id FK
        TEXT message
        TIMESTAMP created_at
        TIMESTAMP edited_at
        TEXT visibility
        TEXT in_reply_to_uri
        TEXT object_uri
        INTEGER federated
        INTEGER sensitive
        TEXT content_warning
        INTEGER reply_count
        INTEGER like_count
        INTEGER boost_count
    }

    follows {
        TEXT id PK
        TEXT account_id FK
        TEXT target_account_id FK
        TEXT uri
        TIMESTAMP created_at
        INTEGER accepted
        INTEGER is_local
    }

    remote_accounts {
        TEXT id PK
        TEXT username
        TEXT domain
        TEXT actor_uri UK
        TEXT display_name
        TEXT summary
        TEXT inbox_uri
        TEXT outbox_uri
        TEXT public_key_pem
        TEXT avatar_url
        TIMESTAMP last_fetched_at
    }

    activities {
        TEXT id PK
        TEXT activity_uri UK
        TEXT activity_type
        TEXT actor_uri
        TEXT object_uri
        TEXT raw_json
        INTEGER processed
        TIMESTAMP created_at
        INTEGER local
        INTEGER reply_count
        INTEGER like_count
        INTEGER boost_count
    }

    likes {
        TEXT id PK
        TEXT account_id FK
        TEXT note_id FK
        TEXT uri
        TIMESTAMP created_at
    }

    delivery_queue {
        TEXT id PK
        TEXT inbox_uri
        TEXT activity_json
        INTEGER attempts
        TIMESTAMP next_retry_at
        TIMESTAMP created_at
        TEXT account_id FK
    }

    hashtags {
        INTEGER id PK
        TEXT name UK
        INTEGER usage_count
        TIMESTAMP last_used_at
    }

    note_hashtags {
        TEXT note_id PK,FK
        INTEGER hashtag_id PK,FK
    }

    note_mentions {
        TEXT id PK
        TEXT note_id FK
        TEXT mentioned_actor_uri
        TEXT mentioned_username
        TEXT mentioned_domain
        TIMESTAMP created_at
    }

    accounts ||--o{ notes : "creates"
    accounts ||--o{ follows : "follower"
    accounts ||--o{ likes : "likes"
    accounts ||--o{ delivery_queue : "owns"
    notes ||--o{ likes : "receives"
    notes ||--o{ note_hashtags : "has"
    notes ||--o{ note_mentions : "mentions"
    hashtags ||--o{ note_hashtags : "used_in"
    remote_accounts ||--o{ follows : "federated_follow"
```

## Tables

### accounts
Local user accounts. Each user authenticates via SSH public key and has an RSA keypair for ActivityPub signing.

### notes
User-created posts. Supports visibility settings, content warnings, threading via `in_reply_to_uri`, and federation status. Includes denormalized engagement counters (`reply_count`, `like_count`, `boost_count`) for efficient display.

### follows
Follow relationships between accounts. Can represent local-to-local, local-to-remote, or remote-to-local follows. The `is_local` flag indicates whether the target is a local user.

### remote_accounts
Cached ActivityPub actors from other servers. Includes public keys for signature verification and inbox URIs for delivery. Cached data has a 24-hour TTL before refresh.

### activities
Log of all ActivityPub activities (incoming and outgoing). Stores raw JSON for debugging and replay. Includes denormalized engagement counters for remote posts displayed in timelines.

### likes
Like/favorite relationships between accounts and notes.

### delivery_queue
Background queue for federating activities to remote servers. Supports retry with exponential backoff (1 minute to 24 hours).

### hashtags
Hashtag registry tracking usage counts for discovery and trending features.

### note_hashtags
Junction table linking notes to their hashtags (many-to-many relationship).

### note_mentions
Stores @username@domain mentions found in notes. Used for notification features and tracking who is mentioned in posts. Mentions are parsed from both local notes and incoming federated activities.

## Indexes

| Table | Index | Columns |
|-------|-------|---------|
| accounts | idx_accounts_username | username (unique, case-insensitive) |
| notes | idx_notes_user_id | user_id |
| notes | idx_notes_created_at | created_at DESC |
| notes | idx_notes_object_uri | object_uri |
| follows | idx_follows_account_id | account_id |
| follows | idx_follows_target_account_id | target_account_id |
| follows | idx_follows_uri | uri |
| remote_accounts | idx_remote_accounts_actor_uri | actor_uri |
| remote_accounts | idx_remote_accounts_domain | domain |
| activities | idx_activities_uri | activity_uri |
| activities | idx_activities_processed | processed |
| activities | idx_activities_type | activity_type |
| activities | idx_activities_created_at | created_at DESC |
| likes | idx_likes_note_id | note_id |
| likes | idx_likes_account_id | account_id |
| delivery_queue | idx_delivery_queue_next_retry | next_retry_at |
| hashtags | idx_hashtags_name | name |
| hashtags | idx_hashtags_usage | usage_count DESC |
| note_hashtags | idx_note_hashtags_note_id | note_id |
| note_hashtags | idx_note_hashtags_hashtag_id | hashtag_id |
| note_mentions | idx_note_mentions_note_id | note_id |
| note_mentions | idx_note_mentions_actor_uri | mentioned_actor_uri |

## Denormalized Counters

For performance optimization, engagement counts are denormalized on both `notes` and `activities` tables:

| Column | Description |
|--------|-------------|
| `reply_count` | Total number of replies (including nested sub-replies, recursively counted) |
| `like_count` | Number of likes/favorites (reserved for future use) |
| `boost_count` | Number of boosts/retweets (reserved for future use) |

These counters are:
- **Incrementally updated** when new replies are received (walks up the ancestor chain)
- **Decremented** when replies are deleted
- **Deduplicated** to avoid counting federated copies of local posts twice
- **Backfilled** during database migration for existing data

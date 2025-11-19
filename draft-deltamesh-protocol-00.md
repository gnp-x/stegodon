# **DeltaMesh: A Git-Based Federation Transport Protocol**

**Internet-Draft**
**Expires: TBD**

---

## **Abstract**

DeltaMesh is a lightweight, Git-based synchronization protocol for small social and microblogging servers.
Instead of delivering activities via HTTP POST requests, DeltaMesh instances exchange content using Git commits and deltas over standard Git transports (SSH, HTTP(S), or local filesystem).

DeltaMesh is designed for small-scale, terminal-oriented communities and provides offline capabilities, strong auditability, and a minimal operational footprint while remaining compatible with ActivityPub-style object models.

---

## **Status of This Memo**

This document is an Internet-Draft and is not an Internet Standards Track specification.
It is published for discussion and experimental implementation.

---

## **Copyright Notice**

This document is placed in the public domain.

---

# **1. Introduction**

DeltaMesh is a distributed synchronization mechanism in which each participating server (“instance”) maintains a Git repository containing posts, activities, and metadata.
Federation occurs through the exchange of Git commits (“deltas”) between instances that are connected in a mesh topology.

DeltaMesh is intended for:

* personal servers
* small federated communities (10–50 instances)
* terminal-first microblogging
* low-latency environments not requiring real-time delivery
* developers who prefer Git and SSH to HTTP infrastructure

DeltaMesh is not intended to replace ActivityPub at global scale.

---

# **2. Terminology**

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**,
**SHOULD**, **SHOULD NOT**, **RECOMMENDED**, **MAY**, and
**OPTIONAL** in this document are to be interpreted as described in RFC 2119.

* **Instance**: A server implementing the DeltaMesh protocol.
* **Mesh**: A set of instances connected through Git remotes.
* **Delta**: A Git commit representing one or more DeltaMesh Activities.
* **Activity**: A unit of social behavior (e.g., Create, Reply, Delete, Follow, Like).
* **Object**: A JSON-encoded entity such as a post (Note).
* **Actor**: A user identity hosted on an instance.

---

# **3. Protocol Overview**

Each instance maintains a single Git repository (the “DeltaMesh repository”).
Instances establish federation links by adding each other as Git remotes.
Instances synchronize state by issuing `git push` and `git fetch` commands.

Activities and objects are encoded as JSON files inside well-defined directory structures and committed with normative commit messages.

Remote instances ingest new commits, extract Activity metadata, and update their local state accordingly.

---

# **4. Repository Structure**

The root of a DeltaMesh repository MUST contain the following directory layout:

```
users/
  <actor-id>/
    actor.json          # Actor profile metadata
    posts/
      <post-id>.json

outbox/
  <activity-id>.json

inbox/
  <instance-id>/
    <activity-id>.json

follows/
  instances/
    <instance-id>.json

blocklists/              # Subscribed blocklist remotes
  <blocklist-id>/
    blocked.json

DELTAMESH_VERSION
```

**Note**: Actor metadata is stored within each user's directory at `users/<actor-id>/actor.json` to maintain a single source of truth and simplify updates.

### 4.1 Version File

`DELTAMESH_VERSION` MUST contain a single integer indicating the protocol version supported by the instance.

Example:

```
1
```

---

# **5. Remotes and Topology**

Each instance MUST maintain **one (1)** Git remote per other instance it follows or communicates with.

Implementations MUST NOT create per-user remotes.

Example:

```
git remote add bob-social git@bob.social:mesh.git
```

This design forms a **mesh topology** rather than a hub-and-spoke model.

## 5.1 Instance Discovery

Instances MUST expose a discovery endpoint at:

```
https://<domain>/.well-known/deltamesh
```

This endpoint MUST return JSON with the following structure:

```json
{
  "version": 1,
  "instance": {
    "domain": "example.social",
    "name": "Example Social",
    "description": "A DeltaMesh instance"
  },
  "git": {
    "url": "git@example.social:mesh.git",
    "protocol": "ssh",
    "publicKey": "ssh-ed25519 AAAA..."
  },
  "capabilities": {
    "activitypub": true,
    "media": "external"
  }
}
```

### 5.1.1 Required Fields

* `version` (REQUIRED) – DeltaMesh protocol version
* `instance.domain` (REQUIRED) – Canonical domain
* `git.url` (REQUIRED) – Git repository URL
* `git.protocol` (REQUIRED) – "ssh" or "https"
* `git.publicKey` (REQUIRED for SSH) – SSH public key for authentication

### 5.1.2 SSH Key Exchange

When adding a remote, instances SHOULD:

1. Query `/.well-known/deltamesh` endpoint
2. Extract `git.publicKey`
3. Add to local `~/.ssh/authorized_keys` or equivalent
4. Add Git remote with provided URL

This enables automated SSH authentication without manual key exchange.

---

# **6. Object Model**

DeltaMesh adopts an ActivityPub-compatible object model.

### 6.1 Posts (Notes)

Files stored under:

`users/<actor-id>/posts/<post-id>.json`

Fields:

* `id` (REQUIRED) – unique post identifier
* `type` (REQUIRED) – MUST be `"Note"`
* `content` (REQUIRED)
* `published` (REQUIRED, ISO-8601)
* `attributedTo` (REQUIRED)
* `inReplyTo` (OPTIONAL) – indicates a Reply
* `url` (OPTIONAL)
* `attachment` (OPTIONAL)

---

# **7. Activities**

Activities MUST be stored as JSON files in:

`outbox/<activity-id>.json`

Supported Activity types:

* **Create**
* **Update**
* **Delete**
* **Follow**
* **Like**
* **Announce** (OPTIONAL)
* **Reply** is represented as a Create with `inReplyTo`.

Activity fields:

* `id` (REQUIRED)
* `type` (REQUIRED)
* `actor` (REQUIRED)
* `object` (REQUIRED)
* `published` (REQUIRED)

---

# **8. Replies**

Replies are encoded using the Note object’s `inReplyTo` property.

When an instance creates a Reply, it MUST:

1. write the Note including `inReplyTo`,
2. create a `Create` Activity referencing that Note,
3. commit changes,
4. push to all remotes.

### 8.1 Thread Reconstruction

Instances MUST reconstruct threads by following `inReplyTo` chains until a root post is reached.

When an instance receives a reply that references a post it doesn't have locally, it SHOULD:

1. Parse the `inReplyTo` URI to identify the source instance
2. Fetch the parent post from that instance's repository
3. Recursively fetch ancestors until the thread root is found
4. Display the complete thread context

Instances SHOULD prefetch entire threads to provide seamless user experience.

Instances MAY display placeholders when referenced posts cannot be fetched.

---

# **9. Commit Format and Publishing**

## 9.1 Manual Publish Workflow

DeltaMesh uses a **manual publish** model similar to Git's staging workflow.

Users MAY create multiple posts, replies, or activities locally without immediately committing them.

When ready to publish, users issue an explicit "publish" command which:

1. Stages all pending activities as JSON files
2. Creates a single commit with metadata
3. Pushes to all configured remotes asynchronously

This workflow provides user control and reduces commit noise.

## 9.2 Commit Message Format

Each commit MUST contain recognizable metadata in the commit message:

```
DELTAMESH:<ActivityType>
actor:<ActorID>
object:<ObjectID>
timestamp:<ISO-8601>
```

Optional fields:

```
inReplyTo:<ObjectID>
target:<ObjectID>
```

## 9.3 Commit Batching

A single commit MAY contain multiple activities if they are published together by the user.

The commit message SHOULD list all activities in the batch.

Example multi-activity commit message:

```
DELTAMESH:Batch
activities:3
timestamp:2025-11-19T22:00:00Z

- DELTAMESH:Create actor:@alice object:post:123
- DELTAMESH:Like actor:@alice object:post:bob:456
- DELTAMESH:Follow actor:@alice target:@bob@other.social
```

---

# **10. Synchronization**

## 10.1 Push Strategy

When publishing, the local instance MUST push to all configured remotes.

Implementations MUST use **asynchronous, parallel pushes** to avoid blocking:

```
for each remote in remotes:
    spawn goroutine/thread:
        git push <remote> main
```

Push operations MUST have a reasonable timeout (RECOMMENDED: 30 seconds).

Failed pushes MUST be queued for retry with exponential backoff.

Push operations MUST NOT block user interface or other operations.

## 10.2 Fetch Strategy

Instances SHOULD periodically fetch from remotes:

```
git fetch <remote>
```

Fetch frequency MUST be configurable per-remote. RECOMMENDED values:

* **Active remotes** (high traffic): every 5-30 minutes
* **Passive remotes** (low traffic): every 2-6 hours
* **Archive remotes**: manual fetch only

Implementations SHOULD allow administrators to configure fetch intervals based on instance activity patterns.

## 10.3 Merge Strategy

After fetching, instances MUST merge new commits:

```
git merge --ff-only <remote>/main
```

If fast-forward merge fails (conflict detected), instances SHOULD:

1. Attempt **timestamp-based resolution**:
   - Compare commit timestamps
   - Accept the commit with earlier timestamp
   - Rebase local commits on top

2. If timestamps are equal or ambiguous:
   - Use lexicographic ordering of instance domains
   - Instance with smaller domain name wins

3. If automatic resolution fails:
   - Log warning
   - Queue for manual resolution
   - Continue with other remotes

Merge conflicts SHOULD be rare in practice due to instance-level isolation.

---

# **11. Delete Semantics and Repository Archival**

## 11.1 Delete Activities

Delete Activities MUST cause the targeted object to be **hidden or tombstoned**.

Instances MUST NOT physically remove Git-tracked files unless explicitly configured, preserving append-only auditability.

## 11.2 Repository Archival Strategy

To manage repository growth, instances SHOULD implement periodic archival:

### 11.2.1 Archive Branches

Posts older than a configured threshold (RECOMMENDED: 6-12 months) SHOULD be moved to archive branches:

```
refs/heads/archive/2024
refs/heads/archive/2023
```

### 11.2.2 Archival Process

1. Create archive branch: `git branch archive/YYYY main`
2. Filter main branch to remove old posts: `git filter-branch`
3. Keep archive branches as historical reference
4. Configure fetch to skip archive branches by default

### 11.2.3 Backfill Configuration

When adding a new remote, administrators SHOULD configure backfill depth:

```
# Full history
git fetch <remote> main

# Last 6 months only
git fetch <remote> main --shallow-since="6 months ago"

# Last 1000 commits
git fetch <remote> main --depth=1000

# Current posts only (no history)
git fetch <remote> main --depth=1
```

Implementations MUST allow per-remote backfill configuration to balance context availability with storage efficiency.

---

# **12. Media Handling**

DeltaMesh uses an **external media** strategy to keep Git repositories lightweight.

## 12.1 Media Storage

Binary media files (images, videos, audio) MUST NOT be stored in the DeltaMesh Git repository.

Instead, posts MUST reference media via external URLs:

```json
{
  "type": "Note",
  "content": "Check out this photo!",
  "attachment": [
    {
      "type": "Image",
      "mediaType": "image/jpeg",
      "url": "https://cdn.example.social/media/abc123.jpg",
      "name": "A beautiful sunset"
    }
  ]
}
```

## 12.2 Media Hosting Options

Instances SHOULD use one of the following strategies:

1. **Self-hosted static files** – Serve from HTTP server outside Git
2. **CDN/S3** – Upload to cloud storage, reference URLs
3. **IPFS** – Decentralized content-addressed storage
4. **External image hosts** – Link to third-party services

## 12.3 Media Synchronization

Media files are NOT synchronized via Git push/pull.

Instances MAY implement optional media mirroring for archival or resilience, but this is outside the scope of the DeltaMesh protocol.

---

# **13. Moderation and Blocklists**

## 13.1 Instance Blocking

Instances MAY block other instances by:

1. Removing the Git remote: `git remote remove <instance>`
2. Adding to local blocklist: `blocklists/local/blocked.json`

Blocked instances' commits are ignored during fetch/merge operations.

## 13.2 Shared Blocklists

Instances MAY subscribe to **shared blocklist repositories** maintained by trusted moderators.

### 13.2.1 Blocklist Repository Structure

A blocklist repository MUST contain:

```
blocked.json    # List of blocked instances
metadata.json   # Blocklist info and maintainer
```

Example `blocked.json`:

```json
{
  "version": 1,
  "updated": "2025-11-19T22:00:00Z",
  "instances": [
    {
      "domain": "spam.example",
      "reason": "Spam",
      "added": "2025-11-01T00:00:00Z"
    },
    {
      "domain": "abuse.example",
      "reason": "Harassment",
      "added": "2025-10-15T00:00:00Z"
    }
  ]
}
```

### 13.2.2 Subscribing to Blocklists

Instances add blocklist repos as Git remotes:

```
git remote add blocklist-main https://github.com/mesh-safety/blocklist.git
```

Fetch periodically:

```
git -C blocklists/mesh-safety fetch origin
```

Apply blocks from subscribed lists when processing activities.

### 13.2.3 Blocklist Governance

Blocklist maintainers SHOULD:

* Document criteria for additions/removals
* Provide appeals process
* Maintain transparency via Git history
* Use GPG-signed commits for authenticity

---

# **14. Security Considerations**

* Git over SSH is RECOMMENDED.
* Instances SHOULD use read-only deploy keys for remotes.
* Activities MAY be signed using GPG or SSH signing.
* Verifying signatures is OPTIONAL but RECOMMENDED.

---

# **13. Optional ActivityPub Interoperability**

Instances MAY expose ActivityPub endpoints such as:

* `/.well-known/webfinger`
* `/users/:id`
* `/objects/:id`

Objects and Activities map directly and losslessly.

---

# **14. IANA Considerations**

This document creates no IANA registries but recommends the future creation of:

* A DeltaMesh Activity Registry
* A DeltaMesh Object Type Registry

---

# **15. Appendix A: Example Commit Message**

```
DELTAMESH:Create
actor:@alice@stegodon.social
object:post:alice:00013
inReplyTo:post:bob:00003
timestamp:2025-11-18T20:01:00Z
```

---

# **16. Appendix B: Example Repository Tree**

```
.
├── users
│   └── alice
│       ├── actor.json                    # Actor profile
│       └── posts
│           ├── 00013.json                # Post object
│           └── 00014.json
├── outbox
│   ├── activity:alice:create:00013.json  # Published activities
│   └── activity:alice:like:00001.json
├── inbox
│   └── bob-social                         # Per-instance inbox
│       ├── activity:bob:create:00003.json
│       └── activity:bob:follow:alice.json
├── follows
│   └── instances
│       ├── bob-social.json                # Remote instance metadata
│       └── carol-tech.json
├── blocklists
│   └── mesh-safety                        # Subscribed blocklist
│       └── blocked.json
└── DELTAMESH_VERSION                      # Protocol version: 1
```

---

# **17. Appendix C: Delta Propagation Flow**

1. **User creates content** – Actor writes one or more posts/activities locally
2. **User triggers publish** – Explicit "publish" command (like `git commit + push`)
3. **Local commit** – Post objects and activities are staged and committed with DeltaMesh metadata
4. **Parallel push** – Instance spawns concurrent push operations to all remotes (non-blocking)
5. **Remote fetch** – Remote instances periodically fetch from all their configured remotes
6. **Conflict resolution** – If needed, use timestamp-based merge strategy
7. **Inbox processing** – Remote instances extract activities from new commits
8. **Thread prefetch** – If activity contains `inReplyTo`, fetch missing parent posts
9. **Timeline update** – Remote instances update local timelines and indexes
10. **User views** – Actors on remote instances see the new content

---

# **18. Appendix D: Complete .well-known/deltamesh Example**

```json
{
  "version": 1,
  "instance": {
    "domain": "stegodon.social",
    "name": "Stegodon Social",
    "description": "A terminal-first microblogging community",
    "admin": {
      "name": "Alice",
      "email": "admin@stegodon.social"
    },
    "stats": {
      "users": 42,
      "posts": 1337,
      "instances": 15
    }
  },
  "git": {
    "url": "git@stegodon.social:mesh.git",
    "protocol": "ssh",
    "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFj7gHV4RFz...",
    "branches": {
      "main": "Current posts and activities",
      "archive/2024": "Archived posts from 2024",
      "archive/2023": "Archived posts from 2023"
    }
  },
  "capabilities": {
    "activitypub": true,
    "media": "external",
    "archival": true,
    "blocklists": ["https://github.com/mesh-safety/blocklist.git"]
  },
  "config": {
    "fetchInterval": {
      "active": "15m",
      "passive": "4h"
    },
    "backfillDefault": "6 months",
    "maxPostAge": "12 months"
  },
  "links": {
    "web": "https://stegodon.social",
    "activitypub": "https://stegodon.social/.well-known/webfinger",
    "docs": "https://docs.stegodon.social"
  }
}
```

---

# **19. Acknowledgments**

This protocol was inspired by Git’s distributed nature and the design goals of small, independent social networks.

---

# **19. Author’s Address**

* [deemkeen](https://github.com/deemkeen) 

---

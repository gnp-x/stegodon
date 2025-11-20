# DeltaMesh: A Git-Based Federation Transport Protocol

Internet-Draft  
Expires: TBD  

---

## Abstract

DeltaMesh is a lightweight, Git-based synchronization protocol for small social and microblogging servers.
Instead of delivering activities via HTTP POST requests, DeltaMesh instances exchange content using Git commits and deltas over standard Git transports (SSH, HTTP(S), or local filesystem).

DeltaMesh is designed for small-scale, terminal-oriented communities and provides offline capabilities, strong auditability, and a minimal operational footprint while remaining compatible with ActivityPub-style object models.

---

## Status of This Memo

This document is an Internet-Draft and is not an Internet Standards Track specification.
It is published for discussion and experimental implementation.

---

## Copyright Notice

This document is placed in the public domain.

---

# 1. Introduction

DeltaMesh is a distributed synchronization mechanism in which each participating server (“instance”) maintains a Git repository containing posts, activities, and metadata.
Federation occurs through the exchange of Git commits (“deltas”) between instances that are connected in a mesh topology.

Unlike protocols that rely on HTTP inboxes and background workers, DeltaMesh uses only Git and a small, well-defined on-disk layout.
This fits naturally into environments where SSH, Git, and text-based tools are already the primary interface.

DeltaMesh is intended for:

- personal servers  
- small federated communities (on the order of 10–50 instances)  
- terminal-first microblogging  
- environments where near-real-time is sufficient and real-time is not required  
- operators who prefer Git and SSH to heavy HTTP infrastructure

DeltaMesh is **not** intended to replace ActivityPub at global scale.
Instead, it offers a complementary, Git-native transport layer that can be bridged to ActivityPub when desired.

---

# 2. Terminology

The key words **MUST**, **MUST NOT**, **REQUIRED**, **SHALL**, **SHALL NOT**,  
**SHOULD**, **SHOULD NOT**, **RECOMMENDED**, **MAY**, and **OPTIONAL**  
in this document are to be interpreted as described in RFC 2119 and RFC 8174.

- **Instance**: A server implementing the DeltaMesh protocol.
- **Mesh**: A set of instances connected through Git remotes.
- **Delta**: A Git commit representing one or more DeltaMesh Activities.
- **Activity**: A unit of social behavior (e.g., Create, Update, Delete, Follow, Like).
- **Object**: A JSON-encoded entity such as a post (Note).
- **Actor**: A user identity hosted on an instance.

---

# 3. Protocol Overview

Each instance maintains a single Git repository (the “DeltaMesh repository”).
Instances establish federation links by adding each other as Git remotes.
Instances synchronize state by issuing `git push` and `git fetch` commands.

Activities and objects are encoded as JSON files inside well-defined directory structures and committed with normative commit messages.
Remote instances ingest new commits, extract Activity metadata, and update their local state accordingly.

DeltaMesh is transport-focused and deliberately small in scope.
It can be used on its own, or as the internal transport layer of a system that also exposes ActivityPub endpoints.

---

# 4. Repository Structure

The root of a DeltaMesh repository MUST contain the following logical layout:

```text
users/
  <actor-id>/
    actor.json
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

blocklists/
  local/
    blocked.json
  mesh/
    <blocklist-id>/

DELTAMESH_VERSION
```

Implementations MAY add additional directories and files, but they MUST NOT change the semantics of the paths defined above.

## 4.1 Version File

`DELTAMESH_VERSION` MUST contain a single integer indicating the protocol version supported by the instance.

Example:

```text
1
```

Future revisions of this document SHOULD increment this version.

---

# 5. Remotes and Topology

Each instance MUST maintain **one (1)** Git remote per other instance it follows or otherwise communicates with.

Implementations MUST NOT create per-user remotes.

Example:

```bash
git remote add bob-social git@bob.social:mesh.git
git remote add carol-net  git@carol.net:mesh.git
```

This design forms a **mesh topology** rather than a hub-and-spoke model.

## 5.1 Instance Discovery

Instances MUST expose a discovery endpoint at:

```text
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

* `version` (REQUIRED) – DeltaMesh protocol version.
* `instance.domain` (REQUIRED) – Canonical domain for this instance.
* `git.url` (REQUIRED) – Git repository URL for the DeltaMesh repository.
* `git.protocol` (REQUIRED) – `"ssh"` or `"https"`.
* `git.publicKey` (REQUIRED for `git.protocol = "ssh"`) – SSH **identity** public key used by this instance when acting as a Git client.

`git.publicKey` is **not** the SSH host key; it is the *instance identity key* used for authenticating **outgoing** Git connections from this instance to other instances.

### 5.1.2 SSH Identity Keys and Trust

Each DeltaMesh instance that uses SSH for Git transport SHOULD maintain a dedicated **instance identity key pair** (e.g. stored as `~deltamesh/.ssh/id_ed25519`), which it uses when acting as a Git client.

The public half of this key MUST be published as `git.publicKey` in the discovery document.

When an administrator of Instance A wants to federate with Instance B, the implementation SHOULD:

1. Perform an HTTPS GET request to:

   ```text
   https://<domain-of-B>/.well-known/deltamesh
   ```

2. Validate the TLS connection in accordance with local policy.

3. Verify that `instance.domain` in the response matches the expected domain for B (or an allowed alias).

4. Read `git.url` and configure a Git remote for B’s DeltaMesh repository:

   ```bash
   git remote add <b-instance-id> <git.url>
   ```

5. Optionally, import B’s `git.publicKey` into a **local trust store** for instance identities.
   This trust store MAY be implemented as:

   * a dedicated `authorized_keys` file for a restricted `deltamesh` SSH account, or
   * an application-specific allow-list that is consulted by a custom Git/SSH front-end.

6. Log the newly trusted instance identity and provide an administrative way to revoke or rotate it.

This allows Instance B to authenticate towards Instance A using its instance identity key when initiating Git connections (for example, if B is configured to push to A, or if A exposes a pull-through cache that requires authentication).

Implementations MAY support a **TOFU (Trust On First Use)** model where the first retrieved `git.publicKey` for a given `instance.domain` is automatically trusted and later changes are flagged for manual review. Implementations SHOULD make such behavior explicit to administrators.

DeltaMesh does **not** mandate fully automatic modification of a system’s global `~/.ssh/authorized_keys`.
Any automation which writes to `authorized_keys` MUST restrict itself to a dedicated account (e.g. `deltamesh`), MUST be clearly documented, and SHOULD be disabled by default in general-purpose distributions.

### 5.1.3 Mutual Federation Setup (Informative)

In a typical bidirectional federation setup between Instance A and Instance B:

1. A fetches `https://B/.well-known/deltamesh`, configures a Git remote for B using `git.url`, and optionally imports B’s `git.publicKey` into A’s instance-trust store.
2. B performs the same procedure against A, reading `https://A/.well-known/deltamesh`.
3. A uses its own instance identity key to authenticate as a Git client when connecting to B’s Git endpoint.
4. B uses its own instance identity key to authenticate as a Git client when connecting to A’s Git endpoint.
5. Both sides can now use `git push` and `git fetch` according to the DeltaMesh synchronization rules, with each instance recognizing the other via the published `git.publicKey` values.

Whether instances rely purely on **pull** (fetching from remotes), purely on **push**, or a combination of both, is an implementation and deployment choice and is outside the strict scope of this discovery mechanism.

---

# 6. Object Model

DeltaMesh adopts an ActivityPub-compatible object model.

## 6.1 Posts (Notes)

Files stored under:

```text
users/<actor-id>/posts/<post-id>.json
```

Fields:

* `id` (REQUIRED) – unique post identifier.
* `type` (REQUIRED) – MUST be `"Note"`.
* `content` (REQUIRED) – textual content.
* `published` (REQUIRED, ISO-8601).
* `attributedTo` (REQUIRED) – actor ID.
* `inReplyTo` (OPTIONAL) – object ID of the parent post (for replies).
* `url` (OPTIONAL).
* `attachment` (OPTIONAL) – media descriptors (see Section 11).

Implementations MAY add additional fields but MUST NOT change the semantics of the defined ones.

---

# 7. Activities

Activities MUST be stored as JSON files in:

```text
outbox/<activity-id>.json
```

Supported Activity types:

* **Create**
* **Update**
* **Delete**
* **Follow**
* **Like**
* **Announce** (OPTIONAL)

A reply is represented as a `Create` Activity whose Note object contains `inReplyTo`.

Common Activity fields:

* `id` (REQUIRED).
* `type` (REQUIRED).
* `actor` (REQUIRED).
* `object` (REQUIRED).
* `published` (REQUIRED, ISO-8601).

Implementations MAY include additional fields as needed.

## 7.1 Replies and Threading

Replies are encoded as `Note` objects with `inReplyTo` set to the ID of the parent post.

When an instance creates a reply, it MUST:

1. write the Note including `inReplyTo`,
2. create a `Create` Activity referencing that Note,
3. commit changes with a DeltaMesh commit message,
4. push to all configured remotes.

### 7.1.1 Thread Reconstruction

Instances SHOULD reconstruct threads by following `inReplyTo` chains from the reply back to a root post.
Instances MUST at least preserve the `inReplyTo` information and MAY display replies even if parent posts are currently missing (e.g., not yet fetched).

Implementations MAY prefetch ancestors when a reply is received and the parent is missing, subject to local policy.

---

# 8. Commit Semantics

## 8.1 One-Activity Commits

The simplest and RECOMMENDED mode is **one Activity per commit**.

## 8.2 Commit Message Format

Each commit MUST contain recognizable metadata in the commit message.

Single-activity commits MUST use the following header:

```text
DELTAMESH:<ActivityType>
actor:<ActorID>
object:<ObjectID>
timestamp:<ISO-8601>
```

Optional fields:

```text
inReplyTo:<ObjectID>
target:<ObjectID>
```

Implementations MUST be able to parse the first line to determine `ActivityType`.
If the first line is not `DELTAMESH:Batch`, the commit MUST be treated as a single-activity commit.

## 8.3 Commit Batching

A single commit MAY contain multiple activities if they are published together by the user.

In this case, the first line of the commit message MUST be:

```text
DELTAMESH:Batch
```

The commit message MUST then include:

* a line `activities:<N>` indicating the number of activities in the batch, and
* one line per activity, each starting with `- DELTAMESH:<ActivityType>` and including enough key-value information to identify `actor`, `object`, and optional `target` / `inReplyTo`.

Example multi-activity commit message:

```text
DELTAMESH:Batch
activities:3
timestamp:2025-11-19T22:00:00Z

- DELTAMESH:Create actor:@alice object:post:123
- DELTAMESH:Like actor:@alice object:post:bob:456
- DELTAMESH:Follow actor:@alice target:@bob@other.social
```

Implementations MUST treat `DELTAMESH:Batch` commits as containing `N` logical Activities, even though they are represented by a single Git commit.

---

# 9. Synchronization

## 9.1 Push Strategy

When publishing, the local instance MUST push the updated branch to all configured remotes.

Implementations SHOULD use asynchronous, parallel pushes to avoid blocking the posting path.
For example:

```text
for each remote in remotes:
    spawn goroutine/thread:
        git push <remote> main
```

Push operations MUST have a reasonable timeout (RECOMMENDED: 10–60 seconds).
On timeout or failure, implementations SHOULD retry with backoff and log the error.

## 9.2 Fetch Strategy

Instances SHOULD periodically fetch from all remotes:

```text
git fetch --all
```

Implementations SHOULD allow different fetch frequencies for different categories of remotes, for example:

* **Active remotes** (recently updated, high interaction): 30–120 seconds.
* **Passive remotes** (infrequently updated): 5–30 minutes.
* **Archive or low-priority remotes**: manual or daily.

Fetch operations SHOULD be resilient to transient errors and MUST NOT corrupt local repositories.

## 9.3 Merge Strategy

After fetching, instances MUST integrate new commits from each remote.

A simple strategy is:

```text
git merge --ff-only <remote>/main
```

If fast-forward merge is not possible (e.g., local divergence), implementations MAY:

* create merge commits, or
* perform manual conflict resolution, or
* temporarily skip the conflicted remote and alert an administrator.

Implementations MUST ensure that the repository remains in a consistent state.
Administrators SHOULD be notified if a remote repeatedly fails to merge.

---

# 10. Delete Semantics and Repository Archival

## 10.1 Delete Activities

Delete Activities MUST cause the targeted object to be **hidden or tombstoned** in local indexes and user interfaces.

Instances MUST NOT physically remove Git-tracked files as part of normal Delete processing, unless explicitly configured by an administrator, in order to preserve append-only auditability.

A Delete Activity SHOULD be represented as:

```json
{
  "id": "activity:alice:delete:00012",
  "type": "Delete",
  "actor": "@alice@stegodon.social",
  "object": "post:alice:00012",
  "published": "2025-11-18T21:00:00Z"
}
```

## 10.2 Repository Archival Strategy

To manage repository growth, instances SHOULD implement strategies to limit the amount of history maintained in active clones.

### 10.2.1 Archive Branches

Instances MAY define archive branches for older content, for example:

* `archive/2023`
* `archive/older-than-1y`

This allows older commits to be isolated from the active branch while still being accessible for offline analysis or backup.

### 10.2.2 Archival Process

Administrators MAY move old commits to archive branches using advanced Git techniques (e.g., `git filter-branch`, `git filter-repo`, or equivalent tools).

Because these operations rewrite history, they:

* MUST be performed with great care,
* SHOULD only be applied to branches that are not actively used by other instances, and
* SHOULD be documented clearly in deployment documentation.

DeltaMesh does **not** require any particular archival mechanism; it only recommends that implementers consider how to keep active history manageable.

### 10.2.3 Backfill Configuration

DeltaMesh-compatible implementations SHOULD provide configuration options controlling how much history is fetched and indexed, such as:

* **Full history**: fetch and index all available commits.
* **Time-based window**: only backfill the last N months.
* **Commit-count window**: only backfill the last N commits.
* **Current-posts-only**: only index recent posts, skipping historical backfill.

These options allow instances to participate in the mesh without having to mirror the full historical state of all peers.

---

# 11. Media Handling

## 11.1 Media Storage

Media attachments (images, audio, video) SHOULD NOT be stored as Git-tracked blobs in the DeltaMesh repository.
Instead, media SHOULD be stored externally and referenced via URLs in Note objects:

```json
"attachment": [
  {
    "type": "Image",
    "mediaType": "image/png",
    "url": "https://media.example.social/alice/123.png"
  }
]
```

## 11.2 Media Hosting Options

Instances MAY choose to host media:

1. On the same domain as the DeltaMesh instance.
2. On a separate media domain (e.g., `media.example.social`).
3. On self-hosted object storage (e.g., S3-compatible).
4. On third-party hosting services.

This choice is deployment-specific and outside the strict scope of DeltaMesh.

## 11.3 Media Synchronization

Media files are NOT synchronized via Git push/pull.

Instances MAY implement optional media mirroring or caching policies (for example, caching media from followed instances), but such mechanisms are considered out of scope for this document.

---

# 12. Moderation and Blocklists

## 12.1 Instance Blocking

Instances MAY block other instances by:

1. Removing the Git remote:

   ```bash
   git remote remove <instance-id>
   ```

2. Adding the instance to a local blocklist file:

   ```text
   blocklists/local/blocked.json
   ```

When an instance is blocked:

* new commits from that remote MUST NOT be merged into local state, and
* any Activities already fetched from that remote MAY be hidden or removed from local indexes, according to local policy.

The origin of a commit SHOULD be associated with the remote from which it was fetched.
Implementations SHOULD treat all commits fetched from a blocked remote as originating from a blocked instance, regardless of the `actor` field.

## 12.2 Shared Blocklists

Instances MAY subscribe to shared blocklist repositories maintained by trusted moderators or communities.

### 12.2.1 Blocklist Repository Structure

A blocklist repository MAY use the following layout:

```text
blocklists/
  mesh-safety/
    metadata.json
    blocked.json
```

Example `metadata.json`:

```json
{
  "id": "mesh-safety",
  "name": "Example Mesh Safety List",
  "maintainer": "admin@example.org",
  "description": "Shared blocklist for abusive instances."
}
```

Example `blocked.json`:

```json
{
  "instances": [
    {
      "domain": "bad.example",
      "reason": "Spam and abuse",
      "addedBy": "admin@example.org",
      "addedAt": "2025-11-20T12:00:00Z"
    }
  ]
}
```

### 12.2.2 Subscribing to Blocklists

To subscribe to a blocklist:

1. Add the blocklist repository as a remote (or clone it separately).
2. Periodically fetch updates.
3. Merge or import `blocked.json` into local moderation policy.

Instances SHOULD allow administrators to:

* enable or disable each blocklist independently,
* override specific entries, and
* review changes before applying them.

### 12.2.3 Blocklist Governance

Blocklist maintainers SHOULD provide:

* clear criteria for inclusion,
* transparent processes for review and appeal, and
* contact information.

Instances SHOULD treat external blocklists as advisory and retain local override capabilities.

---

# 13. Security Considerations

* Git over SSH is RECOMMENDED as the primary transport for DeltaMesh repositories.
* Instances SHOULD use restricted accounts and deploy keys for remote access.
* Activities MAY be signed using GPG or SSH signing to provide authenticity guarantees.
* Verifying signatures is OPTIONAL but RECOMMENDED in high-trust or high-risk environments.
* Administrators SHOULD monitor repository growth, log suspicious Activity patterns, and periodically review remotes and blocklists.
* Use of `git filter-branch` or `git filter-repo` MUST be done carefully to avoid disrupting other instances relying on shared history.

---

# 14. Optional ActivityPub Interoperability

Instances MAY expose ActivityPub-compatible HTTP endpoints such as:

* `/.well-known/webfinger`
* `/users/:id`
* `/objects/:id`
* `/inbox` and `/outbox` (if bridging)

Objects and Activities in DeltaMesh map directly to ActivityPub representations:

* `users/<actor-id>/actor.json` → ActivityPub Actor object.
* `users/<actor-id>/posts/<post-id>.json` → ActivityPub Note object.
* `outbox/<activity-id>.json` → ActivityPub Activity object.

This allows a DeltaMesh instance to:

* federate via Git with other DeltaMesh instances, and
* federate via HTTP with ActivityPub servers, using the same underlying object store.

---

# 15. IANA Considerations

This document requests no IANA actions.

In the future, it MAY be desirable to create:

* a registry of DeltaMesh Activity types, and
* a registry of DeltaMesh capability identifiers for `.well-known/deltamesh`.

---

# 16. Appendix A: Example Commit Message

```text
DELTAMESH:Create
actor:@alice@stegodon.social
object:post:alice:00013
inReplyTo:post:bob:00003
timestamp:2025-11-18T20:01:00Z
```

---

# 17. Appendix B: Example Repository Tree

```text
.
├── users
│   └── alice
│       ├── actor.json                    # Actor object
│       └── posts
│           ├── 00013.json                # Post objects
│           └── 00014.json
├── outbox
│   ├── activity:alice:create:00013.json  # Published activities
│   └── activity:alice:like:00001.json
├── inbox
│   └── bob-social                        # Per-instance inbox
│       ├── activity:bob:create:00003.json
│       └── activity:bob:follow:alice.json
├── follows
│   └── instances
│       ├── bob-social.json               # Remote instance metadata
│       └── carol-tech.json
├── blocklists
│   ├── local
│   │   └── blocked.json                  # Local instance blocklist
│   └── mesh
│       └── mesh-safety
│           ├── metadata.json             # Shared blocklist metadata
│           └── blocked.json              # Shared blocklist entries
└── DELTAMESH_VERSION
```

---

# 18. Appendix C: Delta Propagation Flow

1. Actor creates a post.
2. Post and Activity JSON files are written to `users/.../posts` and `outbox`.
3. Instance commits changes with a DeltaMesh commit message.
4. Instance pushes the updated branch to all remotes.
5. Remote instances periodically fetch from their remotes.
6. Remote instances detect new commits, parse DeltaMesh metadata, and import Activities into `inbox/<instance-id>`.
7. Remote instances update their local timelines and indexes.

---

# 19. Appendix D: Complete .well-known/deltamesh Example

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
      "instances": 12
    }
  },
  "git": {
    "url": "git@stegodon.social:mesh.git",
    "protocol": "ssh",
    "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExamplePublicKey..."
  },
  "capabilities": {
    "activitypub": true,
    "media": "external",
    "blocklists": true
  },
  "links": {
    "homepage": "https://stegodon.social",
    "docs": "https://stegodon.social/docs/deltamesh",
    "source": "https://github.com/deemkeen/stegodon"
  }
}
```

---

# 20. Acknowledgments

This protocol was inspired by Git’s distributed nature and the design goals of small, independent social networks.

---

# 21. Author’s Address

* GitHub: [deemkeen](https://github.com/deemkeen)

---

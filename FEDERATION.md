# ActivityPub federation in Stegodon

Stegodon implements ActivityPub Server-to-Server (S2S) federation, allowing users to follow and be followed by accounts on other Fediverse servers (Mastodon, Pleroma, etc.). Activities are signed with HTTP Signatures and delivered via a background queue with retry logic.

## Supported federation protocols and standards

- [ActivityPub](https://www.w3.org/TR/activitypub/) (Server-to-Server)
- [WebFinger](https://tools.ietf.org/html/rfc7033)
- [HTTP Signatures](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) (RSA-SHA256)
- [NodeInfo 2.0](https://nodeinfo.diaspora.software/)

## Supported activities

### Receiving (Inbox)

- `Follow(Actor)` - Auto-accepted, creates follower relationship
- `Accept(Follow)` - Confirms outgoing follow requests
- `Undo(Follow)` - Removes follower relationship (authorization verified)
- `Create(Note)` - Stores posts from followed accounts (with `inReplyTo` support)
- `Update(Note)` - Updates stored post content
- `Update(Person)` - Re-fetches and caches actor profile
- `Delete(Note)` - Removes stored post (authorization verified)
- `Delete(Actor)` - Removes actor and all associated follows
- `Like` - Acknowledged but not fully processed

### Sending (Outbox)

- `Accept(Follow)` - Sent automatically when receiving Follow
- `Follow(Actor)` - Sent when following a remote user
- `Undo(Follow)` - Sent when unfollowing a remote user
- `Create(Note)` - Delivered to all followers when posting (includes `inReplyTo` for replies)
- `Update(Note)` - Delivered to all followers when editing
- `Delete(Note)` - Delivered to all followers when deleting

## Object types

- `Note` - Primary content type for posts
- `Tombstone` - Received in Delete activities

## Actor types

- `Person` - User accounts

## Collections

- `/users/:username` - Actor profile
- `/users/:username/inbox` - Actor inbox (POST)
- `/users/:username/outbox` - Actor outbox (GET, paginated)
- `/users/:username/followers` - Followers (OrderedCollection)
- `/users/:username/following` - Following (OrderedCollection)
- `/inbox` - Shared inbox (POST)
- `/notes/:id` - Individual note objects

## Discovery

- `/.well-known/webfinger` - WebFinger endpoint (JRD format)
- `/.well-known/nodeinfo` - NodeInfo discovery
- `/nodeinfo/2.0` - NodeInfo 2.0 endpoint

## HTTP Signatures

- Algorithm: `rsa-sha256`
- Signed headers: `(request-target)`, `host`, `date`, `digest`
- Key format: RSA 2048-bit (PKIX/PKCS#8)
- All incoming activities require valid signatures

## Content

- Outgoing posts use `mediaType: text/html`
- Markdown links are converted to HTML anchor tags
- Hashtags are parsed and included in the `tag` array with type `Hashtag`
- Hashtag HTML format: `<a href="..." class="hashtag" rel="tag">#<span>tag</span></a>`
- JSON-LD context includes `Hashtag: as:Hashtag` when hashtags are present
- Incoming content stored as-is in activity JSON

## Replies and Threading

- Replies include the `inReplyTo` field pointing to the parent note's URI
- When replying to a remote user, the parent author's inbox is added to the `cc` list
- Replies are stored with their `in_reply_to_uri` in the database for thread reconstruction
- TUI: Press `r` on a post to reply, press `Enter` to view thread
- Web: Single post pages show parent context and replies section
- Thread depth: 1 level of nesting (direct replies indented, deeper replies shown flat)

## Notable behaviors

- All incoming Follow requests are auto-accepted
- Remote actors are cached for 24 hours
- Delivery queue uses exponential backoff (10 seconds to 24 hours)
- Create activities only accepted from followed accounts
- Rate limiting: 5 requests/second for ActivityPub endpoints
- Maximum activity body size: 1MB

## Not yet implemented

- `Announce` (boost/reblog) activities
- `Like` sending
- Direct messages
- Media attachments
- Content warnings (`sensitive` flag)
- Mentions parsing
- ActivityPub C2S (Client-to-Server)
- Object integrity proofs (FEP-8b32)
- Account migrations
- Shared inbox optimization (currently processes per-actor)

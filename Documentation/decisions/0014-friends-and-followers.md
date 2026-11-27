# ADR 0014: Friends and followers

- **Status:** Proposed
- **Date:** 2026-11-27

## Context

ADR 0009 built the privacy rule (`api/internal/privacy`) around a relationship lookup that only knew Self and None, so Followers & friends and Friends only behaved like Private. Owner decisions: friend requests and follows both need approval (approve/deny, like a private Instagram account); followers are visible to the person they follow, who can remove them; followers are not friends. The `accept_friend_requests` and `accept_follow_requests` toggles from ADR 0009 must be honored when requests are made.

## Decision

### Model (migration 0009)

- **`friendships`**: one row per unordered pair, stored lowest id first (`user_a < user_b`, primary key `(user_a, user_b)`), with `requested_by` (CHECK: one of the pair) and `status` `pending | accepted`. The ordered pair makes "A and B" impossible to store twice in either order. An index on `(user_b, user_a)` serves lookups from the other side.
- **`follows`**: one row per direction, primary key `(follower_id, followee_id)`, CHECK no self-follow, `status` `pending | approved`. An index on `(followee_id, status, follower_id)` serves "who follows me".
- Timestamps `accepted_at` / `approved_at` are set exactly when the status says so (CHECK).
- **Declining, cancelling, unfriending, denying, unfollowing, and removing a follower all delete the row.** There is no "declined" state, so either person can ask again later; the per-user request rate limit (below) stops spam.
- Every foreign key to `users` is `ON DELETE CASCADE`, so `DELETE /me` stays complete (ADR 0009).

### Race safety and idempotency

- Every write is one statement keyed on the pair. Sending twice, accepting twice, or removing something that's gone is a no-op that reports the current state.
- **Crossing friend requests become one friendship.** `SendFriendRequest` first accepts a pending request from the other person; otherwise it inserts with `ON CONFLICT (user_a, user_b) DO UPDATE SET status = 'accepted' WHERE status = 'pending' AND requested_by <> me`. If both people ask at the same moment, the second insert collides on the primary key, waits for the first to commit, and accepts it. An integration test runs the race repeatedly.
- Asking someone who already asked you makes you friends even if they have since turned friend requests off (they asked first).

### The relationship lookup

`store.Relationships.Between(viewer, owner)` returns, closest tie first: **Friend** (accepted friendship), **Follower** (viewer's follow approved), **Pending** (the *owner* has a pending friend or follow request to the viewer), else **None**. A request the viewer sent is None, so asking never unlocks anything. The SQL lives in one fragment (`relationshipSQL`) shared with people search.

`privacy.Pending` is new: `CanViewProfile` lets it see a Private owner's card (the recipient of a request needs to see who's asking), and `CanViewCollections` treats it like a stranger. The full visibility × relationship matrix is a table test.

### API

- **New requests go to a profile and pass the privacy rule**: `POST /users/{handle}/friend-request`, `POST /users/{handle}/follow` use `resolveView`, so a profile the sender can't see (Private, no tie) is 404 with the same body as an unknown handle. Requesting yourself is 400. A request someone has turned off is 403 `"@x isn’t accepting friend requests."` (the profile is already visible to the sender, so this reveals nothing new). Both return `{"relationship": ...}`.
- **Answers and removals act only on your own relationships**: `POST /me/friend-requests/{handle}/accept|decline`, `DELETE /me/friends/{handle}`, `POST /me/follow-requests/{handle}/approve|deny` (deny touches only a pending request), `DELETE /me/followers/{handle}`, `DELETE /users/{handle}/friend-request` (cancel), `DELETE /users/{handle}/follow` (cancel or unfollow). These skip the privacy rule on purpose (you must be able to accept a request from, or remove, someone whose profile you can't see). Removals are always 204, even for an unknown handle; accept/approve are 204 or 404 when there's no such request to you. Neither reveals who exists.
- **Lists**: `GET /me/friends`, `/me/followers`, `/me/following` (`{"people": [...]}`), `GET /me/requests` (incoming and outgoing, both kinds), `GET /me/requests/count` (`{"incoming": n}` for the header badge). A person is only the public card fields (handle, display name, avatar color) plus `since`. Outgoing requests pass `privacy.CanViewProfile`, so someone you asked who has since gone Private drops out of the list.
- **People search**: `GET /people?q=` (signed in; at least 2 characters; 20 results; handle exact/prefix matches first). The SQL narrows candidates and computes each one's relationship with the shared fragment; `privacy.CanViewProfile` makes the final call, so a Private profile you have no tie to never appears, and you never find yourself. Results carry no `since` (no sign-up times). Trigram GIN indexes on `lower(display_name)` and `handle` (migration 0010) keep it off a full scan.
- `GET /users/{handle}` gains `relationship` (null when anonymous or yourself): `friend` (`none | request_sent | request_received | friends`), `following` and `follows_you` (`none | requested | following`), and `accepts_friend_requests` / `accepts_follow_requests`.
- **Rate limits** (per user, in memory, like the handle check): sending friend/follow requests, 30 then one per 10 s; people search, 60 then one per second; 429 with `Retry-After`.

### Web

Profile pages show the friend and follow controls (`RelationshipActions`); unfriend and unfollow ask first. `/people` has search plus Friends, Followers (remove), Following (unfollow), and Requests (accept/decline, approve/deny, cancel) tabs, with the tab in the URL. The header shows a People link with a badge for incoming requests, loaded in the browser after sign-in so SSR and hydration agree.

## Alternatives considered

- **Two rows per friendship** (one per direction). Simpler reads, but the pair could be half-written or duplicated, and crossing requests would need a lock or serializable transaction. The ordered pair makes the database enforce it.
- **Keep declined requests as a row** to block re-requests. Adds state and a "declined" UI nobody asked for; rate limiting plus blocking (below) is the usual answer.
- **Let anyone request a Private profile by handle.** That turns the request endpoint into an existence oracle for Private accounts. A Private user can still make friends by sending requests themselves.
- **Treat the viewer's own pending request as Pending.** It would let asking reveal a Private owner's card. Only the owner reaching out does that.
- **Blocking.** Best practice for social features, but it touches every lookup (hide profiles both ways, refuse requests, drop existing ties, a blocked list). Not built here; deny, remove follower, unfriend, the accept toggles, and rate limits cover the basics. Recorded as a follow-up.

# Review notes: friends-followers

Decisions made without the owner, for review. ADR 0014 (Proposed) has the full model. Owner decisions followed as given: friend and follow requests both need approval; followers are listed for the person they follow, who can remove them; followers are not friends.

## Needs owner decision

- **Blocking.** Not built (see next-steps #2). What's there instead: deny, decline, remove follower, unfriend, the "accept … requests" toggles, Private visibility, and a per-user rate limit on sending requests. Questions for when it's built: should a blocked person be told (most apps: no, they just stop finding you), and should blocking also hide you from their people search and lists (best practice: yes, both ways)?

## Model

- **Declined, cancelled, and removed relationships are deleted, not kept as a "declined" row.** Either person can ask again later. Alternative: keep a declined row to stop re-asking. Rate limiting (30 requests, then one per 10 s, per user) bounds spam; blocking would be the real answer.
- **Friendships are one row per unordered pair** (lowest id first), so crossing requests can't create two rows; the second becomes an acceptance. Alternative: two directed rows, which needs a lock to stay consistent.
- **Asking someone who already asked you makes you friends**, even if they've since turned friend requests off (they asked first).
- **Friend and follow are independent.** A friend can also follow (redundant for visibility, harmless). Unfriending doesn't touch follows, and vice versa.
- **Turning a toggle off doesn't cancel requests already waiting**; it only refuses new ones. Alternative: drop pending requests when the toggle goes off.

## Privacy

- **New `Pending` relationship**: when someone with a Private profile sends you a request, you can see their card (not their collections), so you can decide. A request *you* sent never unlocks anything. Alternative: hide the card and show only name/handle in the requests list (profile link would 404, which felt broken).
- **You can't request a profile you can't see** (a Private profile you have no tie to): it's 404 like an unknown handle, so the request endpoint can't be used to discover Private accounts. A Private user makes friends by sending requests.
- **Refusal when requests are off is 403 with "@x isn't accepting friend requests."** The profile is already visible to the sender, and the profile page shows the same thing ("Not accepting friend requests") instead of the button. Alternative: hide that the toggle is off and silently accept-and-drop. That's dishonest UX.
- **Removals answer 204 even for unknown handles, and accept/approve answer 404 for both "no such user" and "no such request"**, so none of them reveal who exists.
- **Removing a follower doesn't notify them** (copy says so). They can ask again unless follow requests are off.
- **People search requires sign-in, needs 2+ characters, returns 20, and is rate limited** (60, then one per second). It matches handle prefix or display-name substring. It shows any profile whose card you may see (so Friends-only and Followers & friends cards appear). Alternative: only Public profiles in search. Rejected because the card is meant to be findable so requests can reach people (ADR 0009).

## UI

- **People lives at `/people`**, reached from a header icon with a red badge counting incoming requests. The badge refreshes on sign-in, on every navigation, and after each action (no polling or push; real-time sync is deferred).
- **On phones, My collections and People in the header are icons** (with screen-reader labels) so the header fits; text on wider screens.
- **Unfriend, Unfollow, and Remove follower ask for confirmation**; requests, accepts, and cancels don't (easy to redo).
- **Outgoing requests are under Requests → "Sent by you"** rather than a separate tab.
- **No "mutual friends" or "suggested people".** Not asked for.

## Dev seed

- Liam ↔ Noah friends (Liam sees Noah's Friends-only coins), Liam → Amelia approved (Liam sees her Followers & friends plants), Amelia → Liam approved (Liam has a follower), Amelia → Emma pending (Emma has a request). Olivia has no ties so she stays "the stranger" for specs. Emma's request comes from Amelia, not Liam, so Emma's Private profile stays hidden from Liam (an existing spec relies on it).
- The existing profiles e2e that checked "Liam can't see Noah's collections" now checks it as Olivia, and Liam seeing Noah's collection is asserted instead.

## Not finished

- Blocking (above).
- No notifications beyond the badge (email/push are out of scope).

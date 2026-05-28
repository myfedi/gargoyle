# Gargoyle implementation plan

This is the working roadmap for making Gargoyle usable with a Mastodon-compatible UI while preserving clean architecture and safe federation defaults.

## Current state

### Federation / ActivityPub

- [x] WebFinger discovery.
- [x] NodeInfo and host-meta.
- [x] Actor profile at `GET /users/:username`.
- [x] Signed inbox at `POST /users/:username/inbox`.
- [x] Outbox at `GET/POST /users/:username/outbox`.
- [x] Followers/following ActivityPub collections.
- [x] Basic local and inbound Note persistence.
- [x] Signed outbound delivery through durable delivery jobs.
- [x] Inbound `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, `Reject` handling.
- [x] GoToSocial basic compatibility has been validated previously.

### Mastodon-compatible client API

- [x] `POST /api/v1/apps`.
- [x] `GET/POST /oauth/authorize`.
- [x] `POST /oauth/token` with authorization-code + PKCE public clients.
- [x] `GET /api/v1/accounts/verify_credentials`.
- [x] `GET /api/v1/instance` and `GET /api/v2/instance`.
- [x] `POST /api/v1/statuses`.
- [x] `GET /api/v1/timelines/home` and `GET /api/v1/timelines/public` with local/remote filters and pagination.
- [x] `GET /api/v2/search` and `GET /api/v1/accounts/search` for remote account lookup.
- [x] `POST /api/v1/accounts/:id/follow`.
- [x] `GET /api/v1/accounts/relationships`.
- [x] `GET /api/v1/accounts/:id` and `GET /api/v1/accounts/:id/statuses`.
- [x] `GET /api/v1/statuses/:id`, `DELETE /api/v1/statuses/:id`, and `GET /api/v1/statuses/:id/context`.
- [x] Reply posting with `in_reply_to_id` and ActivityPub `inReplyTo`.
- [x] Configurable CORS allowlist for separate browser UI origins.

### Security/configuration

- [x] Secure default remote URL policy: HTTPS + public IPs only.
- [x] Local compatibility exceptions are exact per-host rules via `activitypub.remote_url_exceptions`.
- [x] Inbound inbox signatures required by default.
- [x] Signed POST digest required.
- [x] HTTP clients have timeouts.
- [x] Body limits are configured.

## Priority 1: split Mastodon use cases before adding more

The current `domain/usecases/mastodon.UseCase` is becoming too broad. Split it before implementing more client API workflows.

- [x] Split implementation across focused files for instance/status/timeline/account/relationship workflows.
- [x] Keep HTTP response shape mapping in `infrastructure/web/handlers/mastodon`.
- [x] Keep ActivityPub federation side effects routed through ActivityPub use cases.
- [ ] Optionally split the facade into separate exported use case structs if this package grows further.

## Priority 2: missing Mastodon follow/account endpoints

Needed for real UI social graph flows.

- [x] `POST /api/v1/accounts/:id/unfollow`.
  - [x] Delete outbound following row.
  - [x] Return signed Undo for delivery after commit.
- [x] `GET /api/v1/accounts/:id/followers`.
  - [x] Return local account followers in Mastodon account JSON.
  - [x] Prefer cached remote account profiles and refresh when missing.
- [x] `GET /api/v1/accounts/:id/following`.
  - [x] Return accepted outbound follows.
- [x] `GET /api/v1/accounts/:id`.
- [x] `GET /api/v1/accounts/:id/statuses`.

## Priority 3: remote account cache/read model

Current follow/search can resolve remote actors, but follows mostly store actor URI/inbox. A usable UI needs profile metadata without live fetching on every request.

- [x] Add remote account cache model/table.
- [x] Persist remote actor fields:
  - [x] actor URI
  - [x] username/domain/acct
  - [x] display name
  - [x] summary
  - [x] URL
  - [x] inbox/outbox/followers/following
  - [x] public key
  - [x] fetched_at
- [x] Add repository port for remote account lookup/upsert.
- [x] Make search/follow cache resolved remote accounts.
- [x] Update followers/following endpoints to prefer cached profile data.
- [ ] Add stale-cache refresh policy instead of only refresh-on-miss.

## Priority 4: real home timeline

Current home timeline returns the local user's own notes. Real use needs followed remote posts.

- [x] Ensure inbound Notes from followed actors are stored against the local account timeline.
- [x] Include own posts and followed remote posts.
- [x] Add Mastodon pagination params:
  - [x] `limit`
  - [x] `max_id`
  - [ ] `since_id`
  - [ ] `min_id`
- [x] Add stable ordering by published/created/id.
- [x] Decide current local/public/home semantics.
- [ ] Implement full Mastodon `since_id` / `min_id` behavior.

## Priority 5: durable delivery queue

The current delivery queue is in-memory and not durable.

- [x] Add `delivery_jobs` table:
  - [x] id
  - [x] account_id
  - [x] activity_id
  - [x] inbox_url
  - [x] payload
  - [x] attempts
  - [x] next_attempt_at
  - [x] last_error
  - [x] status
  - [x] created_at / updated_at
- [x] Add delivery job repository port.
- [x] Enqueue jobs from HTTP adapters after use case commits.
- [x] Add worker that processes due jobs.
- [x] Retry with exponential backoff.
- [x] Mark permanent failure/dead-letter after max attempts.
- [x] Document worker operation via admin job inspection command.

## Priority 6: durable fetch queue

Useful for search, actor refresh, missing referenced objects, and remote status hydration.

- [x] Add `fetch_jobs` table.
- [x] Add fetch job repository port.
- [x] Add fetch worker shell with retry/backoff for registered jobs.
- [ ] Queue actor refreshes.
- [x] Queue missing reply-parent object fetches from inbound/outbound notes.
- [x] Persist hydrated remote reply parents as local notes.
- [x] Add de-duplication for repeated pending fetch jobs.

## Priority 7: posting/status compatibility

- [x] `GET /api/v1/statuses/:id`.
- [x] `DELETE /api/v1/statuses/:id`.
- [x] `GET /api/v1/statuses/:id/context`.
- [x] Persist and render `visibility`.
- [x] Persist and render `sensitive`.
- [x] Persist and render `spoiler_text`.
- [x] Support `in_reply_to_id`.
- [x] Media upload and media attachment responses.
- [ ] Fully federate media attachments in ActivityPub payloads.

## Priority 8: OAuth/session polish

- [ ] Improve authorize page copy/UX.
- [ ] Label login as email or username.
- [ ] Add browser session cookie so repeated `/oauth/authorize` does not require password entry.
- [ ] Add token revocation endpoint.
- [ ] Consider refresh tokens if the UI needs long-lived sessions.

## Priority 9: compatibility test loop

- [ ] Test current search/follow/post flows against local GoToSocial again.
- [ ] Test against Mastodon.
- [ ] Test against Akkoma/Pleroma.
- [ ] Record client/UI missing endpoints from actual logs rather than guessing.

## Known hardening limitations

See `LIMITATIONS.md` for security limitations that are intentionally documented but not fully solved yet.

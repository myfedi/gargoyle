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
- [x] Basic signed outbound delivery through an in-memory queue.
- [x] Inbound `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, `Reject` handling.
- [x] GoToSocial basic compatibility has been validated previously.

### Mastodon-compatible client API

- [x] `POST /api/v1/apps`.
- [x] `GET/POST /oauth/authorize`.
- [x] `POST /oauth/token` with authorization-code + PKCE public clients.
- [x] `GET /api/v1/accounts/verify_credentials`.
- [x] `GET /api/v1/instance` and `GET /api/v2/instance`.
- [x] `POST /api/v1/statuses`.
- [x] `GET /api/v1/timelines/home` and `GET /api/v1/timelines/public` basic local timeline.
- [x] `GET /api/v2/search` and `GET /api/v1/accounts/search` for remote account lookup.
- [x] `POST /api/v1/accounts/:id/follow`.
- [x] `GET /api/v1/accounts/relationships`.
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

- [ ] `InstanceUseCase` or `GetInstanceUseCase`.
- [ ] `CreateStatusUseCase`.
- [ ] `TimelineUseCase`.
- [ ] `SearchAccountsUseCase`.
- [ ] `FollowAccountUseCase`.
- [ ] `RelationshipsUseCase`.
- [ ] Keep HTTP response shape mapping in `infrastructure/web/handlers/mastodon`.
- [ ] Keep ActivityPub federation side effects routed through ActivityPub use cases.

## Priority 2: missing Mastodon follow/account endpoints

Needed for real UI social graph flows.

- [ ] `POST /api/v1/accounts/:id/unfollow`.
  - [ ] Add outbound `Undo Follow` use case if needed.
  - [ ] Delete outbound following row.
  - [ ] Deliver signed Undo to remote inbox after commit.
- [ ] `GET /api/v1/accounts/:id/followers`.
  - [ ] Return local account followers in Mastodon account JSON.
  - [ ] Decide whether to fetch live remote actor profiles or use cached remote accounts.
- [ ] `GET /api/v1/accounts/:id/following`.
  - [ ] Return accepted outbound follows.
- [ ] `GET /api/v1/accounts/:id`.
- [ ] `GET /api/v1/accounts/:id/statuses`.

## Priority 3: remote account cache/read model

Current follow/search can resolve remote actors, but follows mostly store actor URI/inbox. A usable UI needs profile metadata without live fetching on every request.

- [ ] Add remote account cache model/table or extend account storage semantics.
- [ ] Persist remote actor fields:
  - [ ] actor URI
  - [ ] username/domain/acct
  - [ ] display name
  - [ ] summary
  - [ ] URL
  - [ ] inbox/outbox/followers/following
  - [ ] public key
  - [ ] fetched_at
- [ ] Add repository port for remote account lookup/upsert.
- [ ] Make search/follow use the cache with refresh-on-stale behavior.
- [ ] Update relationships/followers/following endpoints to return cached profile data.

## Priority 4: real home timeline

Current home timeline returns the local user's own notes. Real use needs followed remote posts.

- [ ] Ensure inbound Notes from followed actors are stored against the local account timeline.
- [ ] Include own posts and followed remote posts.
- [ ] Add Mastodon pagination params:
  - [ ] `limit`
  - [ ] `max_id`
  - [ ] `since_id`
  - [ ] `min_id`
- [ ] Add stable ordering by published/created/id.
- [ ] Decide local/public/home semantics.

## Priority 5: durable delivery queue

The current delivery queue is in-memory and not durable.

- [ ] Add `delivery_jobs` table:
  - [ ] id
  - [ ] account_id
  - [ ] activity_id
  - [ ] inbox_url
  - [ ] payload
  - [ ] attempts
  - [ ] next_attempt_at
  - [ ] last_error
  - [ ] status
  - [ ] created_at / updated_at
- [ ] Add delivery job repository port.
- [ ] Enqueue jobs from use cases after DB commit.
- [ ] Add worker that claims due jobs.
- [ ] Retry with exponential backoff.
- [ ] Mark permanent failure/dead-letter after max attempts.
- [ ] Document worker operation.

## Priority 6: durable fetch queue

Useful for search, actor refresh, missing referenced objects, and remote status hydration.

- [ ] Add `fetch_jobs` table.
- [ ] Add fetch worker with retry/backoff.
- [ ] Queue actor refreshes.
- [ ] Queue missing object/status fetches from inbound activities.

## Priority 7: posting/status compatibility

- [ ] `GET /api/v1/statuses/:id`.
- [ ] `DELETE /api/v1/statuses/:id`.
- [ ] `GET /api/v1/statuses/:id/context`.
- [ ] Support `visibility`.
- [ ] Support `sensitive`.
- [ ] Support `spoiler_text`.
- [ ] Support `in_reply_to_id`.
- [ ] Media upload later.

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

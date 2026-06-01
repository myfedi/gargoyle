# nondogmatic, hackable, pragmatic activitypub server

This project tries to provide a hackable, open to as much federation as possible single user or small user group activitypub server.

Get started with the [documentation](https://github.com/myfedi/gargoyle/tree/main/docs) to get an overview.

## Development

To run it locally, clone the repository, install the Go dependencies, and copy the example config:

```bash
cp config.example.yml config.yml
```

For local development, change the SQLite URI in `config.yml` to a persistent file before running migrations or creating users. The example config uses an in-memory database, which is useful for tests but will not persist across separate CLI/server processes.

```yaml
sqlite:
  uri: "file:gargoyle.db?cache=shared"
```

To init or migrate the database, you can use the db cli tool:

```bash
go run cmd/cli/migrate/main.go db  --config ./config.yml init
go run cmd/cli/migrate/main.go db  --config ./config.yml migrate
```

You can create user by using the admin cli tool:

```bash
go run cmd/cli/admin/main.go register --email test@example.com --username testuser --password 'Str0ngP@ssword!' --config ./config.yml
```

To run the server, run:

```bash
go run cmd/web/server.go ./config.yml
```

## Status

-   [x] webfinger
-   [x] nodeinfo
-   [x] host-meta
-   [x] actor profile
    -   [x] `GET /users/:username`
    -   [x] ActivityPub JSON response
    -   [x] public key in actor document
    -   [x] compatibility-tested against GoToSocial
    -   [ ] compatibility-tested against Mastodon/Akkoma
-   [x] inbox
    -   [x] `POST /users/:username/inbox`
    -   [x] signed request requirement in server mode
    -   [x] stores inbound activities
    -   [x] handles `Follow`
    -   [x] handles `Undo Follow`
    -   [x] handles `Create` for `Note`s
    -   [x] handles `Delete` / `Update` for stored `Note`s
    -   [x] handles `Accept` / `Reject` for outbound follows
    -   [x] handles `Announce` / `Like` notifications for local posts
-   [x] outbox
    -   [x] `GET /users/:username/outbox`
    -   [x] stores local activities
    -   [x] persists local `Note`s
    -   [x] delivers to accepted followers
    -   [x] DB-backed pagination
    -   [x] sanitizes note content
    -   [x] generated stable ULID-based activity/object IDs
    -   [x] persistent delivery queue
    -   [x] auth/user-facing posting API via Mastodon-compatible endpoints
    -   [x] C2S-style `POST /users/:username/outbox` exists in code but is disabled in normal server mode unless explicitly enabled
-   [x] followers/following
    -   [x] followers collection
    -   [x] inbound follow acceptance
    -   [x] outbound follow flow via Mastodon-compatible endpoints
    -   [x] following collection with accepted follows
    -   [x] C2S-style `POST /users/:username/following` exists in code but is disabled in normal server mode unless explicitly enabled

Implemented ActivityPub endpoints:

-   `GET /users/:username` returns an ActivityPub actor.
-   `POST /users/:username/inbox` accepts signed inbound activities and currently handles `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, `Reject`, `Like`, and `Announce`.
-   `GET /users/:username/outbox` returns stored outbox activities.
-   `GET /users/:username/followers` returns accepted followers.
-   `GET /users/:username/following` returns accepted outbound follows.

C2S-style mutation routes, `POST /users/:username/outbox` and `POST /users/:username/following`, are implemented for controlled/test configurations but are not enabled by the normal server wiring. Local posting and following should use the authenticated Mastodon-compatible API unless these routes are explicitly enabled with an authorization story.

## Federation

The server now has the basic pieces for federation and Mastodon-compatible clients: actor discovery, signed inbox requirement, follow acceptance, durable signed delivery jobs, stored local/remote notes, reply threads, OAuth/PKCE login, account search, follow/unfollow, timelines, account profiles, status detail/delete, and followers/following collections.

Compatibility notes:

| Implementation | Discovery | Inbound Follow | Outbound Accept | Outbound Note | Inbound Mention Note | Unfollow |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| GoToSocial | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Mastodon | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ |
| Akkoma/Pleroma | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ |

See [`integration/README.md`](integration/README.md) for the Dockerized GoToSocial integration suite. [`compat/README.md`](compat/README.md) contains the older/manual local compatibility setup and checklist.

### Browser UI hosting and CORS

Mastodon-compatible browser UIs work best when served from the same origin as Gargoyle through a reverse proxy. If the UI is hosted on a separate origin, configure an explicit CORS allowlist:

```yaml
web:
  cors:
    allowed_origins:
      - http://localhost:5173
    allowed_methods: [GET, POST, PUT, PATCH, DELETE, OPTIONS]
    allowed_headers: [Authorization, Content-Type]
    allow_credentials: false
```

Wildcard CORS origins are rejected; only trusted UI origins should be listed.

For local Fediverse compatibility setups that resolve peers such as `gts.test` to loopback/private addresses, opt in with exact per-host exceptions:

```yaml
activitypub:
  remote_url_exceptions:
    - host: gts.test
      allow_http: true
      allow_private_ip: true
```

Do not allow private remote fetching for untrusted production hosts.

Implemented Mastodon-compatible client endpoints include OAuth app registration and authorization-code PKCE, account verification/search/follow/unfollow/relationships/followers/following/profile/statuses, status create/detail/delete/context, media upload/serving, notifications, favourites, boosts, and home/public timelines with local/remote filters.

The GoToSocial integration suite can be run with:

```sh
cd integration/gts
docker compose up -d --build
GARGOYLE_RUN_INTEGRATION=1 go test -v -count=1 .
docker compose down -v --remove-orphans
```

Delivery/fetch jobs can be inspected with:

```sh
go run cmd/cli/admin/main.go jobs --config ./config.yml --type delivery --status failed
go run cmd/cli/admin/main.go jobs --config ./config.yml --type fetch --status pending
```

A background worker automatically removes broken media metadata and old unattached uploads according to `media.cleanup_interval` and `media.unattached_ttl`. The same cleanup can be run manually with:

```sh
go run cmd/cli/admin/main.go media-cleanup --config ./config.yml --older-than 24h
```

Known gaps before claiming broad compatibility:

-   Mastodon/Akkoma compatibility still needs real-world testing.
-   GoToSocial integration coverage includes discovery, follow/unfollow, outbound follow, multiple visibility statuses, direct mentions, favourites, boosts, replies, deletes, OAuth/token setup, and media upload/fetchability, but broader real-server validation is still needed.
-   Fetch and delivery queues have basic observability and duplicate fetch suppression, but need richer operational tooling.
-   Some security limitations remain documented in [`LIMITATIONS.md`](LIMITATIONS.md).

For more, see https://github.com/BasixKOR/awesome-activitypub

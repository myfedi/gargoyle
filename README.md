# nondogmatic, hackable, pragmatic activitypub server

[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=myfedi_gargoyle&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=myfedi_gargoyle)

This project tries to provide a hackable, open to as much federation as possible single user or small user group activitypub server.

Get started with the [documentation](https://github.com/myfedi/gargoyle/tree/main/docs) to get an overview.

## Development

### Prerequisites

Install Go and the local development hooks:

```bash
brew install pre-commit trufflehog
pre-commit install
```

The pre-commit hook runs TruffleHog to catch verified secrets before they are committed.

### Running locally

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

The repository also includes a first-party React/Vite Mastodon-compatible client in `frontend/`. For local UI development, configure `frontend/.env.local` from `frontend/.env.example` and run:

```bash
cd frontend
bun install
bun run dev
```

Before opening a pull request, run the same core checks as CI:

```bash
gofmt -w .
go vet ./...
go test ./...
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
-   [x] followers/following
    -   [x] followers collection
    -   [x] inbound follow acceptance
    -   [x] outbound follow flow via Mastodon-compatible endpoints
    -   [x] following collection with accepted follows

Implemented ActivityPub endpoints:

-   `GET /@:username` redirects to the canonical actor URL.
-   `GET /users/:username` returns an ActivityPub actor.
-   `POST /users/:username/inbox` accepts signed inbound activities and currently handles `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, `Reject`, `Like`, and `Announce`.
-   `GET /users/:username/outbox` returns stored outbox activities.
-   `GET /users/:username/followers` returns accepted followers.
-   `GET /users/:username/following` returns accepted outbound follows.
-   `GET /users/:username/collections/featured` returns the actor's featured collection.

ActivityPub C2S mutation routes are intentionally not exposed. Local posting, following, profile updates, and media management use authenticated Mastodon-compatible API endpoints.

## Federation

The server now has the basic pieces for federation and Mastodon-compatible clients: actor discovery, signed inbox requirement, follow acceptance, durable signed delivery jobs, fetch jobs, stored local/remote notes, reply threads, OAuth/PKCE login, account search, follow/unfollow, timelines, account profiles, profile updates, status create/detail/edit/delete with persisted edit history, favourites, bookmarks, boosts, conversations, notifications, media uploads/serving, and followers/following collections.

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

Implemented Mastodon-compatible client endpoints include:

-   OAuth app registration, authorization-code PKCE, token issuing, and account verification.
-   Instance metadata, account search, profile lookup/update, account statuses, relationships, follow/unfollow, followers, and following.
-   Status create/detail/edit/delete/source/history/context, including persisted edit history, favourites, bookmarks, pins, boosts, replies, and visibility handling.
-   Media upload, metadata update/delete, attachment lookup, and public media serving.
-   Notifications list/clear/dismiss/delete.
-   Conversations list/read/delete.
-   Favourites/bookmarks lists, preferences, custom emojis, trends/lists/filters compatibility responses, and home/public timelines with local/remote filters.
-   A first-party React/Vite frontend that uses this Mastodon-compatible API for login, timelines, compose/reply/edit/delete, media, search, follows, notifications, conversations, and profile management.

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
-   GoToSocial integration coverage includes discovery, follow/unfollow, outbound follow, multiple visibility statuses, direct mentions, status edits/federated Update, favourites, boosts, replies, deletes, OAuth/token setup, and media upload/fetchability, but broader real-server validation is still needed.
-   Fetch and delivery queues have basic observability and duplicate fetch suppression, but need richer operational tooling.
-   Some security limitations remain documented in [`LIMITATIONS.md`](LIMITATIONS.md).

For more, see https://github.com/BasixKOR/awesome-activitypub

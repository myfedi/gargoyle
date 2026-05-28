# nondogmatic, hackable, pragmatic activitypub server

This project tries to provide a hackable, open to as much federation as possible single user or small user group activitypub server.

Get started with the [documentation](https://github.com/myfedi/gargoyle/tree/main/docs) to get an overview.

## Development

To run it locally, just clone the repository, install the go dependencies, `cp config.example.yml config.yml`.

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
    -   [x] `POST /users/:username/outbox`
    -   [x] stores local activities
    -   [x] persists local `Note`s
    -   [x] delivers to accepted followers
    -   [x] DB-backed pagination
    -   [x] sanitizes note content
    -   [x] generated stable ULID-based activity/object IDs
    -   [x] persistent delivery queue
    -   [x] auth/user-facing posting API
-   [ ] followers/following
    -   [x] followers collection
    -   [x] inbound follow acceptance
    -   [x] outbound follow flow
    -   [x] following collection with accepted follows

Implemented ActivityPub endpoints:

-   `GET /users/:username` returns an ActivityPub actor.
-   `POST /users/:username/inbox` accepts signed inbound activities and currently handles `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, `Reject`, `Like`, and `Announce`.
-   `GET /users/:username/outbox` returns stored outbox activities.
-   `POST /users/:username/outbox` creates and stores local `Create`/`Note` activities and delivers them to followers.
-   `GET /users/:username/followers` returns accepted followers.
-   `GET /users/:username/following` returns accepted outbound follows.
-   `POST /users/:username/following` creates and delivers an outbound `Follow`.

## Federation

The server now has the basic pieces for federation and Mastodon-compatible clients: actor discovery, signed inbox requirement, follow acceptance, durable signed delivery jobs, stored local/remote notes, reply threads, OAuth/PKCE login, account search, follow/unfollow, timelines, account profiles, status detail/delete, and followers/following collections.

Compatibility notes:

| Implementation | Discovery | Inbound Follow | Outbound Accept | Outbound Note | Inbound Mention Note | Unfollow |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| GoToSocial | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Mastodon | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ |
| Akkoma/Pleroma | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ | ⏳ |

See [`compat/README.md`](compat/README.md) for the local GoToSocial compatibility setup and the validated flow checklist.

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

Delivery/fetch jobs can be inspected with:

```sh
go run cmd/cli/admin/main.go jobs --config ./config.yml --type delivery --status failed
go run cmd/cli/admin/main.go jobs --config ./config.yml --type fetch --status pending
```

Known gaps before claiming broad compatibility:

-   Mastodon/Akkoma compatibility still needs real-world testing.
-   Media attachments, favourites, boosts, mention delivery, and notifications have Mastodon client API and ActivityPub support, but still need broader real-server validation.
-   Fetch and delivery queues have basic observability and duplicate fetch suppression, but need richer operational tooling.
-   Some security limitations remain documented in [`LIMITATIONS.md`](LIMITATIONS.md).

For more, see https://github.com/BasixKOR/awesome-activitypub

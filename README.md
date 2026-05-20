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
    -   [ ] compatibility-tested against common servers
-   [ ] inbox
    -   [x] `POST /users/:username/inbox`
    -   [x] signed request requirement in server mode
    -   [x] stores inbound activities
    -   [x] handles `Follow`
    -   [x] handles `Undo Follow`
    -   [x] handles `Create` for `Note`s
    -   [x] handles `Delete` / `Update` for stored `Note`s
    -   [x] handles `Accept` / `Reject` for outbound follows
    -   [ ] handles `Announce` / `Like`
-   [ ] outbox
    -   [x] `GET /users/:username/outbox`
    -   [x] `POST /users/:username/outbox`
    -   [x] stores local activities
    -   [x] persists local `Note`s
    -   [x] delivers to accepted followers
    -   [x] DB-backed pagination
    -   [x] sanitizes note content
    -   [x] generated stable ULID-based activity/object IDs
    -   [ ] persistent delivery queue
    -   [ ] auth/user-facing posting API
-   [ ] followers/following
    -   [x] followers collection
    -   [x] inbound follow acceptance
    -   [x] outbound follow flow
    -   [x] following collection with accepted follows

Implemented ActivityPub endpoints:

-   `GET /users/:username` returns an ActivityPub actor.
-   `POST /users/:username/inbox` accepts signed inbound activities and currently handles `Follow`, `Undo Follow`, `Create`, `Delete`, `Update`, `Accept`, and `Reject`.
-   `GET /users/:username/outbox` returns stored outbox activities.
-   `POST /users/:username/outbox` creates and stores local `Create`/`Note` activities and delivers them to followers.
-   `GET /users/:username/followers` returns accepted followers.
-   `GET /users/:username/following` returns accepted outbound follows.
-   `POST /users/:username/following` creates and delivers an outbound `Follow`.

## Federation

The server now has the basic pieces for federation: actor discovery, signed inbox requirement, follow acceptance, signed outbound delivery, stored notes, and outbox/followers collections.

Known gaps before claiming broad compatibility:

-   Mastodon/GoToSocial/Akkoma compatibility still needs real-world testing.
-   Delivery is still in-process; there is no persistent delivery queue yet.
-   Inbox side effects for `Announce` and `Like` are not implemented yet.
-   Outbound follow still needs real-server compatibility testing.

For more, see https://github.com/BasixKOR/awesome-activitypub

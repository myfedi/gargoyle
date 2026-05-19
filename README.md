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
-   [x] inbox, first pass
-   [x] outbox, first pass
-   [x] followers collection, first pass
-   [ ] following/outbound follow flow
-   [ ] delivery queue/retries persistence
-   [ ] compatibility-tested federation with common servers

Implemented ActivityPub endpoints:

-   `GET /users/:username` returns an ActivityPub actor.
-   `POST /users/:username/inbox` accepts signed inbound activities and currently handles `Follow` and `Undo Follow`.
-   `GET /users/:username/outbox` returns stored outbox activities.
-   `POST /users/:username/outbox` creates and stores local `Create`/`Note` activities and delivers them to followers.
-   `GET /users/:username/followers` returns accepted followers.
-   `GET /users/:username/following` currently returns an empty collection.

## Federation

The server now has the basic pieces for federation: actor discovery, signed inbox requirement, follow acceptance, signed outbound delivery, stored notes, and outbox/followers collections.

Known gaps before claiming broad compatibility:

-   Mastodon/GoToSocial/Akkoma compatibility still needs real-world testing.
-   Delivery is still in-process; there is no persistent delivery queue yet.
-   Inbox side effects are limited mostly to `Follow` and `Undo Follow`.
-   Outbound follow/following is not implemented yet.

For more, see https://github.com/BasixKOR/awesome-activitypub

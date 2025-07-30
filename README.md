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
-   [ ] actor
-   [ ] inbox
-   [ ] outbox

## Federation

With which servers can we federate?

-   mastodon: x
-   gotosocial: x
-   akkoma: x
-   pixelfed: x
-   lemmy: x
-   piefed: x
-   bookwyrm: x
-   flohmarkt: x

For more, see https://github.com/BasixKOR/awesome-activitypub

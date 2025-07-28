# nondogmatic, hackable, pragmatic activitypub server

This project tries to provide a hackable, open to as much federation as possible single user or small user group activitypub server.

Get started with the [documentation](https://github.com/myfedi/gargoyle/tree/main/docs) to get an overview.

## Development

To run it locally, just clone the repository, install the go dependencies, `cp config.example.yml config.ym` and run:

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

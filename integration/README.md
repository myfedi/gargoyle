# Gargoyle integration tests

Disposable Docker Compose suites live under one folder per integration target. Services are dockerized; tests are normal local `go test` packages that talk to the Dockerized services over HTTP.

## Plan

1. One folder per target:
   - `gts/` — GoToSocial.
   - `mastodon/` — Mastodon.
   - Future: `wanderer/`, Instagram bridge, book service.
2. Keep each target isolated with its own Compose project, volumes, config, and reverse proxy.
3. Use internal Compose DNS/public hostnames for federation (`gargoyle.test`, `gts.test`, `mastodon.test`) without requiring `/etc/hosts`.
4. Use Go `testing` and API requests for assertions. Add Playwright only for flows that truly need a browser/OAuth UI.
5. Tear down with `-v` to delete all throw-away state.

## Running a suite

From `integration/gts/` or `integration/mastodon/`:

```sh
GARGOYLE_RUN_INTEGRATION=1 go test -v -count=1 .
```

The Go test starts one Compose stack for the package when `GARGOYLE_RUN_INTEGRATION=1` is set, then tears it down at the end. Test data uses unique markers/accounts to avoid requiring expensive per-test Docker resets. GTS accounts are created/promoted by invoking `docker compose exec` against the running `gotosocial` service. Mastodon accounts are created by invoking `rails runner` in the running Mastodon web service. Gargoyle creates its test account in its service entrypoint.

## GoToSocial suite

The suite currently validates:

- OAuth app/token setup against both servers, including GTS authorization-code login.
- GTS resolves Gargoyle via WebFinger/ActivityPub.
- Gargoyle resolves GTS via WebFinger/ActivityPub.
- GTS follows Gargoyle and Gargoyle accepts the signed follow.
- GTS unfollows Gargoyle and Gargoyle removes the follower.
- Gargoyle creates/tracks an outbound follow to GTS.
- Gargoyle delivers public, unlisted, private/followers-only, and direct-mention statuses to GTS.
- GTS delivers public and direct mention statuses to Gargoyle.
- Mentions in both directions.
- GTS favourites and boosts a Gargoyle status.
- GTS unfavourites and unboosts a Gargoyle status.
- Boost visibility matrix for public, unlisted, private, and direct statuses.
- GTS sends a reply activity to a Gargoyle status; Gargoyle context API shows the descendant.
- Gargoyle media upload federates a media URL/status to GTS.
- Gargoyle deletes a status after GTS has received it and deletion propagates.
- Unsigned Gargoyle inbox POST is rejected.
- Private/direct statuses do not leak to a second non-follower GTS account.
- Delivery retry works when GTS is stopped and later restarted.
- Remote URL hardening rejects unconfigured private-host resolution.
- Profile update credentials persist locally and federate to GTS followers.
- Status edits persist locally and federate to GTS via ActivityPub Update.
- Gargoyle-created polls federate to GTS as polls, remote GTS votes are reflected locally, and Gargoyle can vote on a GTS poll through the Mastodon-compatible poll API.
- Fast handler/usecase coverage validates followers-only signed GET dereferencing, inbound Tombstone deletion, inbound Block relationship cleanup, Flag storage/no-op behavior, conservative Move handling, and ActivityPub hashtag/custom emoji tag extraction.

Known compatibility observation captured by the tests: Gargoyle's unlisted status is currently received by GTS as `public`.

Host access goes through the Caddy proxy on `http://127.0.0.1:18080` with Host headers. Inside Docker, public federation hosts are:

- `http://gargoyle.test`
- `http://gts.test`

## Mastodon suite

The Mastodon suite mirrors the core GoToSocial coverage against a disposable Mastodon, PostgreSQL, Redis, and Sidekiq stack. It currently validates:

- OAuth app/password-token setup against both servers.
- Mastodon resolves Gargoyle via WebFinger/ActivityPub.
- Gargoyle resolves Mastodon via WebFinger/ActivityPub.
- Mastodon follows and unfollows Gargoyle.
- Gargoyle creates/tracks an outbound follow to Mastodon.
- Gargoyle delivers public, unlisted, private/followers-only, and direct-mention statuses to Mastodon.
- Mastodon delivers a public mention status to Gargoyle.
- Mastodon favourites and boosts a Gargoyle status.
- Gargoyle deletes a status after Mastodon has received it and deletion propagates.
- Unsigned Gargoyle inbox POST is rejected.
- Profile update credentials persist locally and federate to Mastodon followers.

Host access goes through the Mastodon suite Caddy proxy on `http://127.0.0.1:18081` with Host headers. Inside Docker, public federation hosts are:

- `http://gargoyle.test`
- `http://mastodon.test`

# Gargoyle integration tests

Disposable Docker Compose suites live under one folder per integration target. Services are dockerized; tests are normal local `go test` packages that talk to the Dockerized services over HTTP.

## Plan

1. One folder per target:
   - `gts/` — GoToSocial, implemented first.
   - Future: `mastodon/`, `wanderer/`, Instagram bridge, book service.
2. Keep each target isolated with its own Compose project, volumes, config, and reverse proxy.
3. Use internal Compose DNS/public hostnames for federation (`gargoyle.test`, `gts.test`) without requiring `/etc/hosts`.
4. Use Go `testing` and API requests for assertions. Add Playwright only for flows that truly need a browser/OAuth UI.
5. Tear down with `-v` to delete all throw-away state.

## GoToSocial suite

From `integration/gts/`:

```sh
docker compose up -d --build
GARGOYLE_RUN_INTEGRATION=1 go test -v -count=1 .
docker compose down -v --remove-orphans
```

The Go test starts one Compose stack for the package when `GARGOYLE_RUN_INTEGRATION=1` is set, then tears it down at the end. Test data uses unique markers/accounts to avoid requiring expensive per-test Docker resets. GTS accounts are created/promoted by invoking `docker compose exec` against the running `gotosocial` service. Gargoyle creates its test account in its service entrypoint.

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

Known compatibility observation captured by the tests: Gargoyle's unlisted status is currently received by GTS as `public`.

Host access goes through the Caddy proxy on `http://127.0.0.1:18080` with Host headers. Inside Docker, public federation hosts are:

- `http://gargoyle.test`
- `http://gts.test`

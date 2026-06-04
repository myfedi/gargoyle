# Production deployment

This guide describes the recommended same-origin production deployment for Gargoyle: one public HTTPS origin serves both the browser frontend and the ActivityPub/Mastodon-compatible backend.

## Recommended shape

Use one public origin, for example:

```text
https://example.org
```

Serve the React frontend at `/`, and proxy Gargoyle backend routes on the same origin:

- `/.well-known/*`
- `/nodeinfo*`
- `/users/*`
- `/@*`
- `/api/*`
- `/oauth*`
- `/media*`

This keeps browser API calls same-origin and avoids CORS in production.

## Backend config

Run Gargoyle behind a reverse proxy. Let the proxy terminate TLS and set `public_host` to the public HTTPS URL.

Example `/etc/gargoyle/config.yml`:

```yaml
debug: false
domain: example.org
public_host: https://example.org
port: 8080
tls: false

sqlite:
  uri: "file:/var/lib/gargoyle/gargoyle.db?_pragma=foreign_keys(1)"

media:
  storage_dir: "/var/lib/gargoyle/media"
  cleanup_interval: "1h"
  unattached_ttl: "24h"

web:
  cors:
    allowed_origins: []

activitypub:
  body_limit_bytes: 1048576
  remote_url_exceptions: []
```

`public_host` is important. The backend may listen on local HTTP, but ActivityPub actor IDs, WebFinger links, OAuth redirects, and federation URLs must use the externally visible HTTPS origin.

Do not configure `activitypub.remote_url_exceptions` in production unless you intentionally trust a specific non-public/local test peer.

## Build the frontend

```sh
cd frontend
bun install --frozen-lockfile
bun run build
```

Copy the generated bundle somewhere the reverse proxy can serve it:

```sh
sudo mkdir -p /srv/gargoyle/frontend
sudo rsync -a --delete frontend/dist/ /srv/gargoyle/frontend/dist/
```

## Frontend environment

For same-origin deployment, configure frontend OAuth/API URLs to point at the same public origin.

Example `frontend/.env.production` before building:

```env
VITE_GARGOYLE_API_BASE_URL=https://example.org
VITE_GARGOYLE_OAUTH_AUTHORIZE_URL=https://example.org/oauth/authorize
VITE_GARGOYLE_OAUTH_TOKEN_URL=https://example.org/oauth/token
VITE_GARGOYLE_OAUTH_REDIRECT_URI=https://example.org/
```

Do not put secrets in `VITE_*` variables. They are compiled into browser-visible JavaScript.

## Caddy example

This example serves the static frontend and proxies backend routes to Gargoyle on `127.0.0.1:8080`.

```caddyfile
example.org {
	encode zstd gzip

	root * /srv/gargoyle/frontend/dist

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "no-referrer"
		Permissions-Policy "camera=(), microphone=(), geolocation=()"
		Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests"
	}

	@backend path /.well-known/* /nodeinfo* /users/* /@* /api/* /oauth* /media*
	reverse_proxy @backend 127.0.0.1:8080

	try_files {path} /index.html
	file_server
}
```

The `try_files` fallback is for the React single-page app. Backend routes must be matched before the SPA fallback.

## Run Gargoyle

Build or run the backend with the production config:

```sh
go run cmd/web/server.go /etc/gargoyle/config.yml
```

In a real deployment, run the compiled binary under a service manager such as systemd. Ensure the service user can read/write:

- SQLite database path
- media storage directory
- config file as needed

## Initial database and user

Run migrations:

```sh
go run cmd/cli/migrate/main.go db --config /etc/gargoyle/config.yml init
go run cmd/cli/migrate/main.go db --config /etc/gargoyle/config.yml migrate
```

Create a local user:

```sh
go run cmd/cli/admin/main.go register \
  --config /etc/gargoyle/config.yml \
  --email you@example.org \
  --username alice \
  --password 'use-a-long-unique-password'
```

## Production notes

- Keep the frontend and backend same-origin when possible; leave CORS disabled.
- Terminate TLS at the reverse proxy.
- Keep `public_host` stable. Changing actor URLs after federation can break identity.
- Back up the SQLite database and media directory together.
- Monitor backend logs and delivery/fetch job failures.
- ActivityPub C2S mutation routes are not supported. Local posting/following uses the authenticated Mastodon-compatible API.
- Media is served with backend-level `nosniff` and restrictive media response CSP. A separate media origin can still be used later for additional isolation, but the same-origin deployment above is the simplest supported shape.

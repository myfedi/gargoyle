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
- `/oauth/authorize`
- `/oauth/token`
- `/oauth/revoke`
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

client_api:
  enabled: true
  vapid_public_key: "paste-generated-public-key"
  vapid_private_key: "paste-generated-private-key"
  vapid_subject: "mailto:admin@example.org"

web:
  cors:
    allowed_origins: []

activitypub:
  body_limit_bytes: 1048576
  remote_url_exceptions: []
```

`public_host` is important. The backend may listen on local HTTP, but ActivityPub actor IDs, WebFinger links, OAuth redirects, and federation URLs must use the externally visible HTTPS origin.

Do not configure `activitypub.remote_url_exceptions` in production unless you intentionally trust a specific non-public/local test peer.

For mobile clients such as Ivory, generate one stable VAPID keypair for Mastodon-compatible push notifications and paste it into `client_api.vapid_public_key` / `client_api.vapid_private_key`:

```sh
go run cmd/cli/admin/main.go generate-vapid-keys
```

Keep these keys stable. Rotating them can invalidate existing push subscriptions and require clients to resubscribe. Set `client_api.vapid_subject` to a contact URI, usually `mailto:admin@example.org`.

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

The first-party frontend also needs an OAuth client ID. Register one against the production backend and use the returned `client_id` when building the frontend:

```sh
curl -fsS -X POST https://example.org/api/v1/apps \
  -H 'Content-Type: application/json' \
  --data '{
    "client_name": "Gargoyle Web",
    "redirect_uris": "https://example.org/oauth/callback",
    "scopes": "read write follow push",
    "website": "https://example.org"
  }'
```

Example `frontend/.env.production.local` before building:

```env
VITE_GARGOYLE_API_BASE_URL=https://example.org
VITE_GARGOYLE_OAUTH_CLIENT_ID=returned-client-id
VITE_GARGOYLE_OAUTH_AUTHORIZE_URL=https://example.org/oauth/authorize
VITE_GARGOYLE_OAUTH_TOKEN_URL=https://example.org/oauth/token
VITE_GARGOYLE_OAUTH_REVOKE_URL=https://example.org/oauth/revoke
VITE_GARGOYLE_OAUTH_REDIRECT_URI=https://example.org/oauth/callback
VITE_GARGOYLE_OAUTH_SCOPES="read write follow push"
VITE_GARGOYLE_VAPID_PUBLIC_KEY=generated-vapid-public-key
```

Do not put secrets in `VITE_*` variables. They are compiled into browser-visible JavaScript. The OAuth `client_id` is public; do not put `client_secret` in frontend env.

When building from a working tree used for local development, make sure local env files such as `frontend/.env.local` do not point production builds at development hosts like `gargoyle.test`. Prefer a clean checkout or pass the production `VITE_*` variables explicitly in the build environment.

## Caddy example

This example serves the static frontend and proxies backend routes to Gargoyle on `127.0.0.1:8080`.

```caddyfile
example.org {
	encode zstd gzip

	root * /srv/gargoyle/frontend/dist

	@backend path /.well-known/* /nodeinfo* /users/* /@* /api/* /oauth/authorize /oauth/token /oauth/revoke /media* /inbox
	handle @backend {
		reverse_proxy 127.0.0.1:8080
	}

	handle {
		header {
			Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"
			X-Content-Type-Options "nosniff"
			Referrer-Policy "no-referrer"
			Permissions-Policy "camera=(), microphone=(), geolocation=()"
			Content-Security-Policy "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' https: data: blob:; font-src 'self' data:; connect-src 'self'; media-src 'self' https: blob:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; upgrade-insecure-requests"
		}

		try_files {path} /index.html
		file_server
	}
}
```

The `try_files` fallback is for the React single-page app. Backend routes must be handled before the SPA fallback; use `handle` blocks so routes such as `/oauth/authorize` are not rewritten to `/index.html`. Keep the static frontend CSP inside the static `handle` block so it does not override backend responses such as the OAuth authorization form. A standalone copy of this example is available at [`docs/Caddyfile.production.example`](Caddyfile.production.example).

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

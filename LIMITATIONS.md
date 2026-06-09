# Known Limitations

## HTTP Signature parser

The current HTTP Signature header parser handles the common ActivityPub header shape, but it is intentionally simple. It splits on commas and `=` and does not fully implement quoted-string escaping edge cases.

If interoperability or adversarial malformed signatures become a concern, replace it with a standards-compliant parser.

## Unique constraint classification

Registration maps database uniqueness conflicts by inspecting the database error message. This works for the current SQLite-oriented adapter, but is brittle across databases and drivers.

A cleaner long-term design is adapter-level error classification, e.g. a repository or database error type for unique constraint violations.

## Private key storage

Local actor private keys are currently stored as PEM text in the database. This keeps federation signing simple, but a hardened deployment should use encryption-at-rest or a dedicated key management system.

## RSA signature padding compatibility

Outbound ActivityPub HTTP Signatures currently use `rsa-sha256`, which maps to RSASSA-PKCS1-v1_5 with SHA-256 in widely deployed federation software. RSA-PSS is preferable for new protocols, but using it here would break compatibility with many peers. These operations are signatures, not RSA encryption.

## Outgoing pending follow request listing

Gargoyle persists outgoing follow requests while they are pending, and exposes their per-account state through the Mastodon-compatible relationships endpoint (`/api/v1/accounts/relationships`) as `requested: true`.

It intentionally does not expose pending outgoing follows through `/api/v1/accounts/:id/following`, because that endpoint conventionally represents accepted follows. Mastodon-compatible APIs generally expose incoming follow requests for approval, but do not provide a standard global list of outgoing pending follow requests.

A UI that needs to display pending state should query relationships for accounts it is already rendering, rather than depend on a Gargoyle-specific pending-follow list.

## ActivityPub signed GET dereferencing

Signed GET dereferencing supports public/unlisted objects without a signature and followers-only objects for accepted followers with a valid HTTP signature. Direct-message object dereferencing remains disabled because direct recipient addressing is not yet persisted in a form that can be safely authorized.

## Same-origin OAuth route split

In same-origin deployments, OAuth routes are split between the backend and the frontend SPA:

- backend: `/oauth/authorize`, `/oauth/token`, `/oauth/revoke`
- frontend SPA: `/oauth/callback`

This requires reverse proxies to route only the backend OAuth endpoints to Gargoyle and let `/oauth/callback` fall through to the frontend. A broad matcher such as `/oauth*` will break the callback route; a broad SPA fallback can also accidentally rewrite `/oauth/authorize` to `/index.html`.

Longer term, this should be made less fragile, either by serving the callback under an unambiguous frontend route, serving the first-party UI through backend-aware routing, or documenting/enforcing route ownership more explicitly.

## Remote profile image cache scope

Remote account avatars and headers are cached when a remote account is resolved, and stale profile visits refresh the remote actor document before updating the cached media IDs. The cache is keyed by the remote media URL and is pruned by the remote media cache cleanup policy.

The refresh logic currently relies on the actor document changing its avatar/header URL. It does not perform conditional HTTP revalidation of unchanged media URLs with `ETag`, `Last-Modified`, or `Cache-Control` metadata. If a remote server replaces bytes at the same media URL, Gargoyle may continue serving the older cached copy until the remote media cache is pruned and fetched again.

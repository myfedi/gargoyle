# Known Limitations

## SSRF DNS rebinding protection

Remote ActivityPub actor fetches and inbox deliveries validate the submitted URL and resolved IPs before issuing HTTP requests. This blocks obvious localhost, private, link-local, multicast, and unspecified targets.

A stricter production-grade implementation should validate the IP address actually dialed by a custom `http.Transport.DialContext`, because DNS could theoretically change between preflight validation and connection establishment.

## HTTP Signature parser

The current HTTP Signature header parser handles the common ActivityPub header shape, but it is intentionally simple. It splits on commas and `=` and does not fully implement quoted-string escaping edge cases.

If interoperability or adversarial malformed signatures become a concern, replace it with a standards-compliant parser.

## Unique constraint classification

Registration maps database uniqueness conflicts by inspecting the database error message. This works for the current SQLite-oriented adapter, but is brittle across databases and drivers.

A cleaner long-term design is adapter-level error classification, e.g. a repository or database error type for unique constraint violations.

## Private key storage

Local actor private keys are currently stored as PEM text in the database. This keeps federation signing simple, but a hardened deployment should use encryption-at-rest or a dedicated key management system.

## Outgoing pending follow request listing

Gargoyle persists outgoing follow requests while they are pending, and exposes their per-account state through the Mastodon-compatible relationships endpoint (`/api/v1/accounts/relationships`) as `requested: true`.

It intentionally does not expose pending outgoing follows through `/api/v1/accounts/:id/following`, because that endpoint conventionally represents accepted follows. Mastodon-compatible APIs generally expose incoming follow requests for approval, but do not provide a standard global list of outgoing pending follow requests.

A UI that needs to display pending state should query relationships for accounts it is already rendering, rather than depend on a Gargoyle-specific pending-follow list.

## Direct statuses without recipients

Gargoyle currently accepts Mastodon `visibility=direct` statuses even when the content contains no resolvable mentions. Such a status is stored locally with empty ActivityPub `to` and `cc` recipient lists, which is not useful and may be ignored or rejected by other servers.

A stricter Mastodon-compatible implementation should reject direct statuses unless at least one local or remote mentioned account resolves successfully, or should otherwise require explicit recipients.

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


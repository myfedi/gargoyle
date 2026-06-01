# AGENTS.md

Guidance for coding agents working in this repository.

## Architecture

This project follows a clean architecture / ports-and-adapters style.

### Layers

- `domain/models`: pure domain data structures.
- `domain/ports`: interfaces required by domain/use cases.
- `domain/usecases`: business workflows and application decisions.
- `adapters`: concrete implementations of domain ports.
- `infrastructure`: framework, database, HTTP, config, migrations, and process wiring.
- `cmd`: executable composition roots.

### Dependency rule

Dependencies point inward.

Allowed examples:

- `infrastructure/web/handlers -> domain/usecases`
- `domain/usecases -> domain/ports`
- `adapters/repos -> domain/ports + infrastructure/db/models`
- `cmd/web -> adapters + infrastructure + domain/usecases`

Forbidden examples:

- `domain/* -> infrastructure/*`
- `domain/* -> adapters/*`
- use cases importing HTTP/Fiber/database driver packages directly
- handlers performing multi-step business workflows that belong in use cases

Handlers should parse/serialize HTTP, authenticate, call use cases, and map errors. They should not own business rules.

## Transactions

Use cases own transaction boundaries for multi-write workflows.

- Use `domain/ports/db.TxProvider.RunInTx`.
- Do not manually call `Commit` or `Rollback` in domain code.
- Network delivery must happen after commit, not inside DB transactions.

## Ports and adapters

When domain logic needs an external capability, add a port under `domain/ports` and implement it in `adapters` or `infrastructure`.

Examples:

- content sanitization: `domain/ports.ContentSanitizer`
- ID generation: `domain/ports.IDGenerator`
- ActivityPub network/signature operations: `domain/ports/activitypub`

Do not import utility or infrastructure packages into domain use cases just because it is convenient.

## Constructor validation

Required dependencies should be validated in constructors. Panicking during startup wiring is acceptable because missing dependencies are non-recoverable configuration/programming errors.

Use clear panic messages, e.g.:

```go
panic("mastodon API use case requires NotesRepo")
```

## Security defaults

Prefer secure-by-default behavior. If a behavior is unsafe for production, it must be disabled by default and require an explicit, clearly named opt-in.

- Public routes must never mutate local account state unless they authenticate the local user/account first. ActivityPub C2S is not currently supported; do not add local outbox/following mutation routes unless C2S support is explicitly requested and designed with local authentication/authorization.
- Inbound ActivityPub inbox writes require signatures.
- Unsigned inbox processing must be explicit opt-in only.
- Signed POST bodies require a signed `Digest` header.
- Inbound ActivityPub mutation activities must prove ownership: e.g. `Create` note `attributedTo`, `Update`, and `Delete` objects must match the verified activity actor.
- HTTP remote ActivityPub URLs are disabled by default and must be config-gated.
- Remote URL fetching/delivery must validate schemes, hosts, redirects, private IP ranges, and the IP address actually dialed by the HTTP transport. Preflight DNS checks alone are not sufficient.
- HTTP clients must have timeouts.
- Request body limits must be configured and enforced by Fiber; handler-level checks are acceptable as defense in depth. Size-limited readers must reject over-limit bodies, not silently truncate them.
- File/media upload endpoints must use an allowlist of safe content types, reject active content such as HTML/SVG, serve with `X-Content-Type-Options: nosniff`, and avoid same-origin executable content where possible.
- OAuth/bearer-token handling must include expiration, scope enforcement on protected routes, brute-force/rate-limit protection on credential endpoints, and hashed-at-rest bearer tokens. Plaintext tokens are only returned at issue time.
- Do not leak raw internal errors through HTTP responses.
- Production composition roots must not depend on `mock` packages or placeholder adapters.

Known limitations should be documented in `LIMITATIONS.md` rather than hidden in code comments.

## ActivityPub rules

- Use cases own ActivityPub workflows.
- HTTP signature verification and delivery are infrastructure/adapters behind ports.
- Persist local side effects in a transaction, then deliver after commit.
- Validate actor ownership for inbound mutation activities like `Create`, `Update`, and `Delete`.
- Validate `Accept`/`Reject` Follow objects against the original follow relationship.

## Mastodon-compatible API rules

Mastodon client API compatibility is separate from federation.

- OAuth workflows live in `domain/usecases/oauth`.
- Mastodon client API workflows live in `domain/usecases/mastodon`.
- HTTP routes and JSON response shapes live in `infrastructure/web/handlers/mastodon`.
- Reuse ActivityPub use cases for actions that affect federation, e.g. creating a status should go through the outbox workflow.
- Add only the Mastodon API surface required by tested clients, but keep response shapes compatible enough for real UIs.

## Error handling

Use `domain/models/domainerrors` in use cases.

- Bad user input -> `ErrBadRequest`
- Auth failures -> `ErrUnauthorized`
- Missing resources -> `ErrNotFound`
- Unexpected repository/adapter/system failures -> `ErrInternal`

HTTP handlers should call `web.HandleDomainError`.

## Testing and validation

Before finishing changes, run:

```sh
go test ./...
```

Also run `gofmt` on touched Go files.

For federation/client compatibility changes, prefer adding focused tests or documenting manual compatibility checks.

## Git hygiene

Keep commits surgical and task-focused.

- Do not stage unrelated untracked files.
- Preserve local user files like `PLAN.md`, `frontend/`, or local config unless explicitly asked.
- Prefer small commits with imperative messages.

# ActivityPub core / client API architecture

Gargoyle separates federation workflows from local client/API workflows.

## Ownership

- `domain/usecases/activitypub` owns ActivityPub-core workflows, AP document construction, federation mutation validation, persistence side effects, and delivery payload construction.
- `domain/usecases/clientapi` owns local client/product workflows: accounts, statuses as client commands, timelines, interactions, notifications, conversations, media, profile, and moderation admin actions.
- Mastodon compatibility is an HTTP wire-format concern under `infrastructure/web/handlers/clientapi`; it is not the domain model.

## Composition

Runtime composition remains in `infrastructure/server`:

- `MountDiscovery`
- `MountActivityPub`
- `MountClientAPI`
- `StartCoreWorkers`

Client API workflow composition uses narrow workflow structs and narrow config structs. Do not reintroduce a broad client API facade/use case, a single exported mega config, or a shared dependency bag hidden behind helper structs.

## Client API workflow groups

- `Instance`
- `Accounts`
- `Statuses`
- `Timelines`
- `Interactions`
- `Notifications`
- `Conversations`
- `Media`
- `Profile`
- `Moderation`

Handlers may mount one client API surface, but implementation files and dependencies should stay grouped by workflow area.

## ActivityPub core boundaries

AP-core mutations live in `activitypub`, including:

- follow / undo follow
- like / announce / undo
- delete status/object
- create status metadata persistence
- update status/object
- update actor/profile
- accept/reject follow request
- poll voting
- remote object hydration
- remote outbox collection caching
- AP status create/update document builders

## Intentionally outside AP core

- Bookmarks and pins are local client/product state.
- Admin moderation endpoints/job enqueue are client/admin surface; AP core enforces domain blocks.
- Media upload/update/delete is local client storage policy; AP core consumes existing media attachments.

## Validation

```sh
gofmt -w <touched go files>
go test ./...
```

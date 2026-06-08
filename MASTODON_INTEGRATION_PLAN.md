# Mastodon integration completion plan

## Goal
Bring `integration/mastodon` close to parity with the GoToSocial suite for high-value federation flows, plus important locked-account and DM edge cases.

## Work items

1. Add reusable helpers
   - `ensureGargoyleFollowsMastodon` improvements if needed.
   - unique Mastodon account setup/token helper for extra users.
   - media upload helper and 1x1 PNG fixture.
   - generic status absence and conversation/status access helpers.
   - compose helper for outage/retry tests.

2. Add parity tests
   - Media attachment federation: upload media to Gargoyle, post attached status, assert Mastodon receives attachment or URL.
   - Reply/thread context: Gargoyle root received by Mastodon, Mastodon reply received in Gargoyle context.
   - Status edit federation: edit Gargoyle status and assert Mastodon remote copy updates.
   - Poll federation/voting: Gargoyle poll received by Mastodon; Mastodon vote reflected locally; Mastodon poll received by Gargoyle; Gargoyle vote works.
   - Unfavourite/unboost propagation: Mastodon favourite/unfavourite and reblog/unreblog Gargoyle status.
   - Boost visibility matrix: public/unlisted boostable, private/direct not boostable.
   - Private/direct non-leak: follower sees private/direct, second non-follower does not.
   - Delivery retry after Mastodon outage.
   - Remote URL hardening rejects unconfigured private hosts.
   - Mastodon normal follower delivery to Gargoyle without an explicit mention.

3. Add extra high-value cases
   - Gargoyle -> locked Mastodon follow-request reject.
   - Multi-recipient DMs and non-leak for unmentioned user.
   - Direct status without a valid local mention is rejected/not delivered.
   - Locked profile update federation both ways where supported by APIs/ActivityPub.

4. Validate
   - `gofmt` touched Go files.
   - `go test ./...`.
   - `cd integration/mastodon && GARGOYLE_RUN_INTEGRATION=1 go test -v -count=1 .`.

## Notes
Some cases may expose product compatibility gaps. If a gap is real, keep the test focused and either fix the product behavior or document the limitation before committing.

# Federation hardening brief

This brief tracks the next six production-readiness items for ActivityPub/Fediverse interoperability. Each item should be implemented with secure defaults, clean architecture, focused tests, and no public mutation routes without local authentication.

## 1. Dereferenceable local ActivityPub resources

Remote peers commonly fetch ActivityPub IDs after receiving deliveries. Local `Note` object IDs, `Create` activity IDs, and follow activity IDs must resolve to ActivityPub JSON when safe to expose. Public unauthenticated dereferencing must not leak followers-only or direct content; private object authorization can be added later with signed GET support.

## 2. Shared inbox

Expose and advertise a shared inbox so peers can fan out less and use common delivery paths. The shared inbox should reuse the same signature, digest, body-limit, domain-block, and transaction rules as per-user inbox delivery.

## 3. Inbound Undo completeness

Handle `Undo` for `Like` and `Announce` in addition to `Follow`, removing local interaction/boost state and associated notifications where appropriate. Ownership must be validated from the embedded activity actor.

## 4. Production wiring for inbound boosts

Ensure the inbox use case receives the boost repository in the production composition root so inbound `Announce` activities produce the same persisted timeline/read-model state as tested use-case paths.

## 5. Production wiring for missing reply fetches

Ensure user inbox processing receives the fetch-job repository so missing `inReplyTo` parents are queued consistently from real inbound federation traffic.

## 6. Wider ActivityPub vocabulary support

Add only the vocabulary we can model correctly. Likely next targets are `Question`/polls, actor `Move`, moderation/report activities such as `Flag`, and richer tag handling for hashtags/custom emoji. Unsupported activity types should remain safely accepted/stored or ignored without corrupting local state.

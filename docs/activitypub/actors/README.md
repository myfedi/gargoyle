# Actors, Users, and Accounts

Actors: Entities capable of performing activities. Defined by the spec. Types include Person, Service, Group, Application.

-   Accounts: Implementation concept. Usually refers to local actors under your control (e.g. a user signed up on your instance). Remote actors from other servers may be cached/stored locally, but they aren’t full accounts in your system.

So:

-   All accounts are actors.
-   Not all actors are accounts.
-   Local accounts = local actors
-   Remote actors = federated entities, optionally stored as "remote accounts" in local DBs for interaction.

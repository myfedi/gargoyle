CREATE TABLE remote_accounts (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fetched_at DATETIME NOT NULL,
    username TEXT NOT NULL,
    domain TEXT,
    display_name TEXT,
    summary TEXT,
    uri TEXT NOT NULL UNIQUE,
    url TEXT,
    inbox_uri TEXT NOT NULL,
    outbox_uri TEXT,
    following_uri TEXT,
    followers_uri TEXT,
    featured_collection_uri TEXT,
    public_key TEXT,
    actor_type INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE accounts (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    user_id CHAR(26) NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fetched_at DATETIME,

    username TEXT NOT NULL,
    domain TEXT,
    display_name TEXT,
    summary TEXT,
    uri TEXT NOT NULL UNIQUE,
    url TEXT,
    inbox_uri TEXT,
    outbox_uri TEXT,
    following_uri TEXT,
    followers_uri TEXT,
    featured_collection_uri TEXT,
    private_key TEXT,
    public_key TEXT NOT NULL UNIQUE,

    actor_type INTEGER NOT NULL DEFAULT 0,

    UNIQUE(username, domain),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

--bun:split

CREATE TABLE users (
	id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	password_hash TEXT NOT NULL,
	email TEXT UNIQUE,
	admin BOOLEAN NOT NULL DEFAULT FALSE
);
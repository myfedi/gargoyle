CREATE TABLE activities (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    local_account_id CHAR(26) NOT NULL,
    direction TEXT NOT NULL,
    type TEXT NOT NULL,
    actor TEXT NOT NULL,
    object TEXT,
    raw_json TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

--bun:split

CREATE INDEX activities_local_account_direction_idx ON activities(local_account_id, direction, created_at);

--bun:split

CREATE TABLE follows (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    local_account_id CHAR(26) NOT NULL,
    remote_actor TEXT NOT NULL,
    remote_inbox TEXT,
    activity_id CHAR(26) NOT NULL,
    accepted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(local_account_id, remote_actor),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
);

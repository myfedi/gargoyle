CREATE TABLE follows_new (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    local_account_id CHAR(26) NOT NULL,
    remote_actor TEXT NOT NULL,
    remote_inbox TEXT,
    activity_id CHAR(26) NOT NULL,
    direction TEXT NOT NULL DEFAULT 'follower',
    accepted_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    UNIQUE(local_account_id, remote_actor, direction),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
);

--bun:split

INSERT INTO follows_new (id, local_account_id, remote_actor, remote_inbox, activity_id, direction, accepted_at, created_at)
SELECT id, local_account_id, remote_actor, remote_inbox, activity_id, 'follower', accepted_at, created_at FROM follows;

--bun:split

DROP TABLE follows;

--bun:split

ALTER TABLE follows_new RENAME TO follows;

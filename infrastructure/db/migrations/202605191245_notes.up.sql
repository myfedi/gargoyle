CREATE TABLE notes (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    local_account_id CHAR(26) NOT NULL,
    activity_id CHAR(26) NOT NULL UNIQUE,
    uri TEXT NOT NULL UNIQUE,
    content TEXT NOT NULL,
    plain_text TEXT,
    attributed_to TEXT NOT NULL,
    published_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (activity_id) REFERENCES activities(id) ON DELETE CASCADE
);

--bun:split

CREATE INDEX notes_local_account_published_idx ON notes(local_account_id, published_at);

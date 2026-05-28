CREATE TABLE status_interactions (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    note_id CHAR(26) NOT NULL,
    type TEXT NOT NULL,
    UNIQUE(local_account_id, note_id, type),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
);

--bun:split

CREATE TABLE notifications (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    actor_account_id CHAR(26) NOT NULL,
    type TEXT NOT NULL,
    status_id CHAR(26),
    read_at DATETIME,
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

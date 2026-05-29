CREATE TABLE mentions (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    note_id CHAR(26) NOT NULL,
    account_id TEXT NOT NULL,
    username TEXT NOT NULL,
    acct TEXT NOT NULL,
    url TEXT NOT NULL,
    UNIQUE(note_id, account_id),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX idx_mentions_note ON mentions(note_id);

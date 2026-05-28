CREATE TABLE boosts (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    actor TEXT NOT NULL,
    note_id CHAR(26) NOT NULL,
    uri TEXT NOT NULL UNIQUE,
    published_at DATETIME NOT NULL,
    UNIQUE(local_account_id, actor, note_id),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX idx_boosts_timeline ON boosts(local_account_id, published_at DESC, id DESC);
CREATE INDEX idx_boosts_note ON boosts(note_id);

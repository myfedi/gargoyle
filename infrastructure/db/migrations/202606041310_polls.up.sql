ALTER TABLE notes ADD COLUMN poll_multiple BOOLEAN NOT NULL DEFAULT 0;

--bun:split

ALTER TABLE notes ADD COLUMN poll_expires_at DATETIME;

--bun:split

CREATE TABLE poll_options (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    note_id CHAR(26) NOT NULL,
    title TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    votes_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
);

--bun:split

CREATE UNIQUE INDEX poll_options_note_position_idx ON poll_options(note_id, position);

--bun:split

CREATE TABLE poll_votes (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    poll_option_id CHAR(26) NOT NULL,
    note_id CHAR(26) NOT NULL,
    local_account_id CHAR(26),
    remote_actor TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (poll_option_id) REFERENCES poll_options(id) ON DELETE CASCADE,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

--bun:split

CREATE UNIQUE INDEX poll_votes_local_option_idx ON poll_votes(poll_option_id, local_account_id) WHERE local_account_id IS NOT NULL;

--bun:split

CREATE UNIQUE INDEX poll_votes_remote_option_idx ON poll_votes(poll_option_id, remote_actor) WHERE remote_actor IS NOT NULL;

--bun:split

CREATE INDEX poll_votes_note_local_idx ON poll_votes(note_id, local_account_id);

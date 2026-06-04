ALTER TABLE notes ADD COLUMN edited_at DATETIME;

CREATE TABLE status_edit_history (
    id CHAR(26) NOT NULL PRIMARY KEY,
    note_id CHAR(26) NOT NULL,
    content TEXT NOT NULL,
    plain_text TEXT,
    visibility TEXT NOT NULL DEFAULT 'public',
    sensitive BOOLEAN NOT NULL DEFAULT FALSE,
    spoiler_text TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX idx_status_edit_history_note_created ON status_edit_history(note_id, created_at);

CREATE TABLE status_edit_history_media (
    edit_id CHAR(26) NOT NULL,
    media_id CHAR(26) NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (edit_id, media_id),
    FOREIGN KEY (edit_id) REFERENCES status_edit_history(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media_attachments(id) ON DELETE CASCADE
);

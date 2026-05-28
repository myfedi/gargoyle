CREATE TABLE note_media_attachments (
    note_id CHAR(26) NOT NULL,
    media_id CHAR(26) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (note_id, media_id),
    FOREIGN KEY (note_id) REFERENCES notes(id) ON DELETE CASCADE,
    FOREIGN KEY (media_id) REFERENCES media_attachments(id) ON DELETE CASCADE
);

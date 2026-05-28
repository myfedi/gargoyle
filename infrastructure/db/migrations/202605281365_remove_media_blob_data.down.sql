CREATE TABLE media_attachments_old (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    file_name TEXT,
    content_type TEXT NOT NULL,
    storage_path TEXT NOT NULL DEFAULT '',
    data BLOB NOT NULL DEFAULT X'',
    description TEXT,
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

--bun:split

INSERT INTO media_attachments_old (id, created_at, updated_at, local_account_id, file_name, content_type, storage_path, description)
SELECT id, created_at, updated_at, local_account_id, file_name, content_type, storage_path, description FROM media_attachments;

--bun:split

DROP TABLE media_attachments;

--bun:split

ALTER TABLE media_attachments_old RENAME TO media_attachments;

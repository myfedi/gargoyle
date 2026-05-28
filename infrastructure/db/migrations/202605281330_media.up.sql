CREATE TABLE media_attachments (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id CHAR(26) NOT NULL,
    file_name TEXT,
    content_type TEXT NOT NULL,
    data BLOB NOT NULL,
    description TEXT,
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

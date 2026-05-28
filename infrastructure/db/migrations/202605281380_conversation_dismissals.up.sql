CREATE TABLE conversation_dismissals (
    local_account_id CHAR(26) NOT NULL,
    conversation_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (local_account_id, conversation_id),
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE
);

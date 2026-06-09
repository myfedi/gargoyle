DELETE FROM oauth_access_tokens WHERE user_id IS NULL OR user_id = '';

PRAGMA foreign_keys=off;

CREATE TABLE oauth_access_tokens_old (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    application_id CHAR(26) NOT NULL,
    user_id CHAR(26) NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    scopes TEXT NOT NULL,
    expires_at DATETIME,
    FOREIGN KEY (application_id) REFERENCES oauth_applications(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO oauth_access_tokens_old (id, created_at, updated_at, application_id, user_id, token_hash, scopes, expires_at)
SELECT id, created_at, updated_at, application_id, user_id, token_hash, scopes, expires_at FROM oauth_access_tokens;

DROP TABLE oauth_access_tokens;
ALTER TABLE oauth_access_tokens_old RENAME TO oauth_access_tokens;

PRAGMA foreign_keys=on;

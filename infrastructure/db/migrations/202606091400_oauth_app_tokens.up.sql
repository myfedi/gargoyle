PRAGMA foreign_keys=off;

CREATE TABLE oauth_access_tokens_new (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    application_id CHAR(26) NOT NULL,
    user_id CHAR(26),
    token_hash TEXT NOT NULL UNIQUE,
    scopes TEXT NOT NULL,
    expires_at DATETIME,
    FOREIGN KEY (application_id) REFERENCES oauth_applications(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

INSERT INTO oauth_access_tokens_new (id, created_at, updated_at, application_id, user_id, token_hash, scopes, expires_at)
SELECT id, created_at, updated_at, application_id, user_id, token_hash, scopes, expires_at FROM oauth_access_tokens;

DROP TABLE oauth_access_tokens;
ALTER TABLE oauth_access_tokens_new RENAME TO oauth_access_tokens;

PRAGMA foreign_keys=on;

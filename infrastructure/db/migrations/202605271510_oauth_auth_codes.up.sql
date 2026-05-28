CREATE TABLE oauth_authorization_codes (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    application_id CHAR(26) NOT NULL,
    user_id CHAR(26) NOT NULL,
    code_hash TEXT NOT NULL UNIQUE,
    redirect_uri TEXT NOT NULL,
    scopes TEXT NOT NULL,
    code_challenge TEXT,
    code_challenge_method TEXT,
    expires_at DATETIME NOT NULL,
    used_at DATETIME,
    FOREIGN KEY (application_id) REFERENCES oauth_applications(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

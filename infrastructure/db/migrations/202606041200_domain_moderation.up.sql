CREATE TABLE domain_blocks (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    domain TEXT NOT NULL UNIQUE,
    severity TEXT NOT NULL DEFAULT 'suspend',
    reject_media BOOLEAN NOT NULL DEFAULT TRUE,
    public_comment TEXT,
    private_comment TEXT,
    created_by_user_id CHAR(26) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE CASCADE
);

--bun:split

CREATE INDEX idx_domain_blocks_domain ON domain_blocks(domain);

--bun:split

CREATE TABLE moderation_jobs (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    last_error TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    finished_at DATETIME
);

--bun:split

CREATE INDEX idx_moderation_jobs_due ON moderation_jobs(status, next_attempt_at);

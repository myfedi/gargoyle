CREATE TABLE delivery_jobs (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    account_id CHAR(26) NOT NULL,
    activity_id CHAR(26),
    inbox_url TEXT NOT NULL,
    payload BLOB NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    last_error TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    delivered_at DATETIME
);

--bun:split

CREATE INDEX idx_delivery_jobs_due ON delivery_jobs(status, next_attempt_at);

--bun:split

CREATE TABLE fetch_jobs (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    url TEXT NOT NULL,
    kind TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    last_error TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    fetched_at DATETIME
);

--bun:split

CREATE INDEX idx_fetch_jobs_due ON fetch_jobs(status, next_attempt_at);

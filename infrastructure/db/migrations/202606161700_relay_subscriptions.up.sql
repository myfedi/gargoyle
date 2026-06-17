CREATE TABLE relay_subscriptions (
    id CHAR(26) PRIMARY KEY NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    actor_uri TEXT NOT NULL UNIQUE,
    inbox_uri TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    accepted_at DATETIME,
    created_by_user_id CHAR(26) NOT NULL,
    last_error TEXT,
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_relay_subscriptions_status ON relay_subscriptions(status);

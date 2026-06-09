CREATE TABLE push_subscriptions (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    local_account_id TEXT NOT NULL,
    access_token_id TEXT NOT NULL UNIQUE,
    endpoint TEXT NOT NULL,
    key_p256dh TEXT NOT NULL,
    key_auth TEXT NOT NULL,
    policy TEXT NOT NULL DEFAULT 'all',
    alert_mention BOOLEAN NOT NULL DEFAULT 1,
    alert_status BOOLEAN NOT NULL DEFAULT 0,
    alert_reblog BOOLEAN NOT NULL DEFAULT 1,
    alert_follow BOOLEAN NOT NULL DEFAULT 1,
    alert_follow_request BOOLEAN NOT NULL DEFAULT 1,
    alert_favourite BOOLEAN NOT NULL DEFAULT 1,
    alert_poll BOOLEAN NOT NULL DEFAULT 1,
    alert_update BOOLEAN NOT NULL DEFAULT 0,
    alert_admin_sign_up BOOLEAN NOT NULL DEFAULT 0,
    alert_admin_report BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (local_account_id) REFERENCES accounts(id) ON DELETE CASCADE,
    FOREIGN KEY (access_token_id) REFERENCES oauth_access_tokens(id) ON DELETE CASCADE
);

CREATE INDEX idx_push_subscriptions_local_account_id ON push_subscriptions(local_account_id);

CREATE TABLE push_delivery_jobs (
    id TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    subscription_id TEXT NOT NULL,
    notification_id TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL,
    last_error TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    delivered_at DATETIME,
    FOREIGN KEY (subscription_id) REFERENCES push_subscriptions(id) ON DELETE CASCADE,
    FOREIGN KEY (notification_id) REFERENCES notifications(id) ON DELETE CASCADE
);

CREATE INDEX idx_push_delivery_jobs_due ON push_delivery_jobs(status, next_attempt_at);
CREATE UNIQUE INDEX idx_push_delivery_jobs_unique ON push_delivery_jobs(subscription_id, notification_id);

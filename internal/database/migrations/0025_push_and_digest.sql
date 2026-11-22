-- +goose Up

-- Web Push rest alerts (gap 4). One row per browser PushSubscription. The
-- endpoint is unique: a device that signs in as someone else moves to that
-- user. Keys are the subscription's public p256dh key and auth secret; the
-- server never holds the browser's private key.
CREATE TABLE push_subscriptions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint        TEXT    NOT NULL UNIQUE,
    p256dh          TEXT    NOT NULL,
    auth            TEXT    NOT NULL,
    user_agent      TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_success_at DATETIME
);

CREATE INDEX idx_push_subscriptions_user ON push_subscriptions(user_id);

-- The VAPID application-server key pair, generated on first start when
-- REPLOG_VAPID_PUBLIC_KEY / REPLOG_VAPID_PRIVATE_KEY are not set, so
-- existing subscriptions survive restarts. The private key is stored
-- encrypted ("enc:" prefix) when REPLOG_SECRET_KEY is set. Single row.
CREATE TABLE vapid_keys (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    public_key  TEXT NOT NULL,
    private_key TEXT NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Weekly coach digest (gap 8): one row per coach per digest week
-- (week_start is that week's Monday, YYYY-MM-DD) so a restart or a second
-- scheduler pass never sends the same week twice.
CREATE TABLE weekly_digest_log (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    week_start TEXT    NOT NULL,
    sent_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, week_start)
);

-- +goose Down

DROP TABLE IF EXISTS weekly_digest_log;
DROP TABLE IF EXISTS vapid_keys;
DROP INDEX IF EXISTS idx_push_subscriptions_user;
DROP TABLE IF EXISTS push_subscriptions;

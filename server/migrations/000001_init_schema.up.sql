-- Enable pgcrypto for gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Users
CREATE TABLE users (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    apple_sub  TEXT        NOT NULL UNIQUE,
    email      TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Devices (APNs tokens)
CREATE TABLE devices (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    apns_token TEXT        NOT NULL UNIQUE,
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_user_id ON devices(user_id);

-- Refresh tokens
CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id  UUID        REFERENCES devices(id) ON DELETE SET NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Trips
CREATE TABLE trips (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    origin_lat           DOUBLE PRECISION NOT NULL DEFAULT 0,
    origin_lng           DOUBLE PRECISION NOT NULL DEFAULT 0,
    origin_name          TEXT        NOT NULL DEFAULT '',
    destination_lat      DOUBLE PRECISION NOT NULL,
    destination_lng      DOUBLE PRECISION NOT NULL,
    destination_name     TEXT        NOT NULL,
    desired_arrival_at   TIMESTAMPTZ NOT NULL,
    warning_minutes      INTEGER     NOT NULL DEFAULT 0,
    status               TEXT        NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active','notified','cancelled','expired')),
    latest_eta_seconds   INTEGER     NOT NULL DEFAULT 0,
    prev_eta_seconds     INTEGER     NOT NULL DEFAULT 0,
    stable_eta_count     INTEGER     NOT NULL DEFAULT 0,
    next_poll_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    notification_sent_at TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_trips_user_id      ON trips(user_id);
CREATE INDEX idx_trips_next_poll    ON trips(next_poll_at) WHERE status = 'active';
CREATE INDEX idx_trips_status       ON trips(status);

-- Notification logs
CREATE TABLE notification_logs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    trip_id         UUID        NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    device_id       UUID        REFERENCES devices(id) ON DELETE SET NULL,
    apns_message_id TEXT        NOT NULL DEFAULT '',
    payload         JSONB       NOT NULL DEFAULT '{}',
    status          TEXT        NOT NULL DEFAULT 'sent'
                        CHECK (status IN ('sent','failed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notif_logs_trip_id ON notification_logs(trip_id);

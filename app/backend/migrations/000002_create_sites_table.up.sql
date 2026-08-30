CREATE TABLE sites (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    label TEXT,
    check_interval_sec INT NOT NULL DEFAULT 60,
    timeout_sec INT NOT NULL DEFAULT 10,
    failure_threshold INT NOT NULL DEFAULT 2,
    retry_count INT NOT NULL DEFAULT 1,
    expected_status_min INT NOT NULL DEFAULT 200,
    expected_status_max INT NOT NULL DEFAULT 399,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_unique_user_normalized_url_active
ON sites (user_id, normalized_url)
WHERE deleted_at IS NULL;
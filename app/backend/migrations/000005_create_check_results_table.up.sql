CREATE TABLE check_results (
    id BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    checked_at TIMESTAMPTZ NOT NULL,
    status_code INT,
    response_time_ms INT,
    is_up BOOLEAN NOT NULL,
    error_type TEXT,
    error_message TEXT,
    timing_breakdown JSONB
);
CREATE TABLE site_state (
    site_id UUID PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
    current_status TEXT NOT NULL CHECK (current_status IN ('up','down','pending')),
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_checked_at TIMESTAMPTZ,
    last_status_change_at TIMESTAMPTZ,
    uptime_pct_24h NUMERIC(5,2),
    uptime_pct_30d NUMERIC(5,2)
);
CREATE TABLE notification_events (
    id UUID PRIMARY KEY,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    transition TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivery_status TEXT
);
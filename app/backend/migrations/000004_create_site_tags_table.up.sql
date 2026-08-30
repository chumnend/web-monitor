CREATE TABLE site_tags (
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (site_id, tag_id)
);
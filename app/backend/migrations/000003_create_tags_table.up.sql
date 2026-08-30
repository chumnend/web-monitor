CREATE TABLE tags (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    UNIQUE (user_id, name)
);
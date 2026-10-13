-- +goose Up
ALTER TABLE events ADD COLUMN mode TEXT NOT NULL DEFAULT 'online';
ALTER TABLE events ADD COLUMN date TIMESTAMPTZ;

UPDATE events SET date = created_at WHERE date IS NULL;
ALTER TABLE events ALTER COLUMN date SET NOT NULL;
ALTER TABLE events ALTER COLUMN mode DROP DEFAULT;

CREATE TABLE event_attendees (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event_id, user_id)
);

CREATE INDEX idx_event_attendees_user_id ON event_attendees (user_id);

CREATE TABLE connections (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_a_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_b_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    method TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (user_a_id, user_b_id),
    CHECK (user_a_id < user_b_id)
);

CREATE INDEX idx_connections_user_b ON connections (user_b_id);

CREATE TABLE connection_requests (
    id UUID PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    from_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    to_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    responded_at TIMESTAMPTZ,
    UNIQUE (event_id, from_user_id, to_user_id)
);

CREATE INDEX idx_connection_requests_to_user ON connection_requests (to_user_id);

CREATE TABLE event_qr_tokens (
    token_hash TEXT PRIMARY KEY,
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_event_qr_tokens_event_user ON event_qr_tokens (event_id, user_id);
CREATE INDEX idx_event_qr_tokens_expires_at ON event_qr_tokens (expires_at);

CREATE TABLE connection_scans (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    scanner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scanned_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event_id, scanner_id, scanned_id)
);

-- +goose Down
DROP TABLE connection_scans;
DROP TABLE event_qr_tokens;
DROP TABLE connection_requests;
DROP TABLE connections;
DROP TABLE event_attendees;
ALTER TABLE events DROP COLUMN mode;
ALTER TABLE events DROP COLUMN date;

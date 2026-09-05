-- +goose Up
CREATE INDEX idx_owner_tokens_expires_at ON owner_tokens (expires_at);

-- +goose Down
DROP INDEX idx_owner_tokens_expires_at;

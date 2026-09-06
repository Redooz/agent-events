-- +goose Up
ALTER TABLE owners RENAME TO users;
ALTER TABLE owner_tokens RENAME TO user_tokens;

ALTER TABLE agents RENAME COLUMN owner_id TO user_id;
ALTER TABLE events RENAME COLUMN owner_id TO user_id;
ALTER TABLE user_tokens RENAME COLUMN owner_id TO user_id;

ALTER INDEX idx_owner_tokens_owner_id RENAME TO idx_user_tokens_user_id;
ALTER INDEX idx_owner_tokens_expires_at RENAME TO idx_user_tokens_expires_at;
ALTER INDEX idx_agents_owner_id RENAME TO idx_agents_user_id;
ALTER INDEX idx_events_owner_id RENAME TO idx_events_user_id;

ALTER TABLE users RENAME CONSTRAINT owners_pkey TO users_pkey;
ALTER TABLE users RENAME CONSTRAINT owners_provider_subject_key TO users_provider_subject_key;
ALTER TABLE user_tokens RENAME CONSTRAINT owner_tokens_pkey TO user_tokens_pkey;
ALTER TABLE user_tokens RENAME CONSTRAINT owner_tokens_owner_id_fkey TO user_tokens_user_id_fkey;
ALTER TABLE agents RENAME CONSTRAINT agents_owner_id_fkey TO agents_user_id_fkey;
ALTER TABLE events RENAME CONSTRAINT events_owner_id_fkey TO events_user_id_fkey;

-- +goose Down
ALTER TABLE users RENAME CONSTRAINT users_pkey TO owners_pkey;
ALTER TABLE users RENAME CONSTRAINT users_provider_subject_key TO owners_provider_subject_key;
ALTER TABLE user_tokens RENAME CONSTRAINT user_tokens_pkey TO owner_tokens_pkey;
ALTER TABLE user_tokens RENAME CONSTRAINT user_tokens_user_id_fkey TO owner_tokens_owner_id_fkey;
ALTER TABLE agents RENAME CONSTRAINT agents_user_id_fkey TO agents_owner_id_fkey;
ALTER TABLE events RENAME CONSTRAINT events_user_id_fkey TO events_owner_id_fkey;

ALTER INDEX idx_user_tokens_user_id RENAME TO idx_owner_tokens_owner_id;
ALTER INDEX idx_user_tokens_expires_at RENAME TO idx_owner_tokens_expires_at;
ALTER INDEX idx_agents_user_id RENAME TO idx_agents_owner_id;
ALTER INDEX idx_events_user_id RENAME TO idx_events_owner_id;

ALTER TABLE agents RENAME COLUMN user_id TO owner_id;
ALTER TABLE events RENAME COLUMN user_id TO owner_id;
ALTER TABLE user_tokens RENAME COLUMN user_id TO owner_id;

ALTER TABLE users RENAME TO owners;
ALTER TABLE user_tokens RENAME TO owner_tokens;

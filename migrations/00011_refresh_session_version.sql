-- +goose Up
ALTER TABLE refresh_tokens ADD COLUMN session_version bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE refresh_tokens DROP COLUMN session_version;

-- +goose Up
ALTER TABLE users ADD COLUMN must_change_password boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN session_version bigint NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE users DROP COLUMN session_version;
ALTER TABLE users DROP COLUMN must_change_password;

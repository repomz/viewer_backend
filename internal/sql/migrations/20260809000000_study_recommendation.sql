-- +goose Up
ALTER TABLE studies
    ADD COLUMN IF NOT EXISTS recommendation TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE studies
    DROP COLUMN IF EXISTS recommendation;

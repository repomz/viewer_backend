-- +goose Up
ALTER TABLE login_events ADD COLUMN event_id UUID UNIQUE;
-- +goose Down
ALTER TABLE login_events DROP COLUMN event_id;

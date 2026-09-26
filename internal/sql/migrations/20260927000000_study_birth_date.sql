-- +goose Up
ALTER TABLE studies ADD COLUMN birth_date DATE;

-- +goose Down
ALTER TABLE studies DROP COLUMN birth_date;

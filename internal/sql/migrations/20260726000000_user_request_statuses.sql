-- +goose Up
ALTER TABLE user_requests
    DROP CONSTRAINT IF EXISTS user_requests_status_check;

UPDATE user_requests SET status = 'in_progress' WHERE status = 'in_process';
UPDATE user_requests SET status = 'error' WHERE status = 'failed';

ALTER TABLE user_requests
    ADD CONSTRAINT user_requests_status_check
    CHECK (status IN ('pending', 'in_progress', 'completed', 'error'));

-- +goose Down
ALTER TABLE user_requests
    DROP CONSTRAINT IF EXISTS user_requests_status_check;

UPDATE user_requests SET status = 'in_process' WHERE status = 'in_progress';
UPDATE user_requests SET status = 'failed' WHERE status = 'error';

ALTER TABLE user_requests
    ADD CONSTRAINT user_requests_status_check
    CHECK (status IN ('pending', 'in_process', 'completed', 'failed'));

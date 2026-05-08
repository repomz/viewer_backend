-- +goose Up
-- +goose StatementBegin
CREATE TABLE books (
    id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at 		timestamp with time zone NOT NULL DEFAULT now(),
    updated_at 		timestamp with time zone NOT NULL DEFAULT now(),
    body text  NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE books;
-- +goose StatementEnd

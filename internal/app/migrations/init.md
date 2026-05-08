-- +goose Up
CREATE TABLE books (
    id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at 		timestamp with time zone NOT NULL DEFAULT now(),
    updated_at 		timestamp with time zone NOT NULL DEFAULT now(),
    name text  NOT NULL
);


-- +goose Down
DROP TABLE books;


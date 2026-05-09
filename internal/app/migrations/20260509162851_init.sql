-- +goose Up
-- +goose StatementBegin
CREATE TABLE studies (
    id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at 		timestamp with time zone NOT NULL DEFAULT now(),
    updated_at 		timestamp with time zone NOT NULL DEFAULT now(),
	study_id        text  NOT NULL,
	patient        text  NOT NULL,
	age            integer,
	department     text  NOT NULL,
	name_operation  text  NOT NULL,
	descr_operation text  NOT NULL,
	time_beginning  timestamp,
	time_duration   integer,
	surgeon        text  NOT NULL,
	dicom_link      text
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE studies;
-- +goose StatementEnd

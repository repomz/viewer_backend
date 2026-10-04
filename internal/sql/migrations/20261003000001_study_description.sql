-- +goose Up
-- descr_operation remains the physical legacy conclusion column for rolling upgrades.
ALTER TABLE studies ADD COLUMN description TEXT NOT NULL DEFAULT '';
COMMENT ON COLUMN studies.descr_operation IS 'Conclusion; exposed as conclusion in the API';

-- +goose Down
ALTER TABLE studies DROP COLUMN description;

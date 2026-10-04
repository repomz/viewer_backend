-- +goose Up
ALTER TABLE studies ADD COLUMN room INTEGER NOT NULL DEFAULT 0 CHECK(room IN (0,1,2));
ALTER TABLE agent_logs ADD COLUMN has_issues BOOLEAN GENERATED ALWAYS AS
  (content ~ '\|[[:space:]]*(WARNING|ERROR|CRITICAL)[[:space:]]*\|') STORED;
CREATE INDEX agent_logs_issues_period ON agent_logs(period_start DESC) WHERE has_issues;

-- +goose Down
DROP INDEX agent_logs_issues_period;
ALTER TABLE agent_logs DROP COLUMN has_issues;
ALTER TABLE studies DROP COLUMN room;

-- +goose Up
ALTER TABLE studies ADD COLUMN options TEXT NOT NULL DEFAULT ''
  CHECK (options = '' OR options ~ '^(ivus|vabk|ekmo)(,(ivus|vabk|ekmo))*$');
ALTER TABLE studies ADD COLUMN payment TEXT NOT NULL DEFAULT '' CHECK (payment IN ('', 'oms', 'vmp'));
ALTER TABLE studies ADD COLUMN payment_pending BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE studies ADD COLUMN payment_override TEXT CHECK (payment_override IN ('oms', 'vmp'));
ALTER TABLE app_users ADD COLUMN can_confirm_payment BOOLEAN NOT NULL DEFAULT false;
UPDATE app_users SET can_confirm_payment = true WHERE lower(login) = 'artem';
INSERT INTO app_users (display_name, login, password_hash, quota_bytes)
VALUES ('Иван', 'suslov', '$2y$12$uqu8ZjbKT0P0eD7zzhpubehoTYuI9tzVQ6Ku8orIiqTyxTUGSf3AO', 1073741824)
ON CONFLICT (lower(login)) DO NOTHING;

CREATE TABLE payment_rules (
  field TEXT NOT NULL CHECK (field IN ('study_type', 'options')),
  value TEXT NOT NULL CHECK (btrim(value) <> ''),
  mode TEXT NOT NULL CHECK (mode IN ('auto', 'review')),
  PRIMARY KEY (field, value)
);
CREATE TABLE payment_decisions (
  id BIGSERIAL PRIMARY KEY,
  study_id UUID NOT NULL REFERENCES studies(id),
  user_id BIGINT NOT NULL REFERENCES app_users(id),
  payment TEXT NOT NULL CHECK (payment IN ('oms', 'vmp')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementBegin
CREATE FUNCTION resolve_study_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE automatic BOOLEAN; review BOOLEAN;
BEGIN
  IF NEW.time_beginning IS NULL OR
     EXTRACT(YEAR FROM (NEW.time_beginning AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Tomsk') <>
     EXTRACT(YEAR FROM now() AT TIME ZONE 'Asia/Tomsk') THEN
    NEW.payment_pending := false;
    RETURN NEW;
  END IF;
  IF TG_OP = 'UPDATE' AND (NEW.study_type IS DISTINCT FROM OLD.study_type OR NEW.options IS DISTINCT FROM OLD.options) THEN
    NEW.payment_override := NULL;
  END IF;
  SELECT coalesce(bool_or(mode='auto'),false), coalesce(bool_or(mode='review'),false)
    INTO automatic, review FROM payment_rules
    WHERE (field='study_type' AND value=NEW.study_type)
       OR (field='options' AND value = ANY(string_to_array(replace(NEW.options, ';', ','), ',')));
  NEW.payment_pending := false;
  IF NEW.payment_override IS NOT NULL THEN NEW.payment := NEW.payment_override;
  ELSIF automatic THEN NEW.payment := 'vmp';
  ELSIF review THEN NEW.payment := 'oms'; NEW.payment_pending := true;
  ELSE NEW.payment := 'oms';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER studies_resolve_payment BEFORE INSERT OR UPDATE ON studies
FOR EACH ROW EXECUTE FUNCTION resolve_study_payment();
UPDATE studies SET payment='oms'
WHERE EXTRACT(YEAR FROM (time_beginning AT TIME ZONE 'UTC') AT TIME ZONE 'Asia/Tomsk') =
      EXTRACT(YEAR FROM now() AT TIME ZONE 'Asia/Tomsk')
  AND lower(study_type) NOT IN ('xa','ct');

-- +goose Down
DROP TRIGGER studies_resolve_payment ON studies;
DROP FUNCTION resolve_study_payment();
DROP TABLE payment_decisions;
DROP TABLE payment_rules;
ALTER TABLE studies DROP COLUMN options, DROP COLUMN payment, DROP COLUMN payment_pending, DROP COLUMN payment_override;
ALTER TABLE app_users DROP COLUMN can_confirm_payment;

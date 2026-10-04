-- Run only against a disposable database after migrations.
BEGIN;
INSERT INTO payment_rules(field,value,mode) VALUES ('study_type','каг','auto');
INSERT INTO studies(study_id,patient,department,name_operation,study_type,descr_operation,surgeon,time_beginning,room)
VALUES ('test-current','Test Current','test','КАГ','каг','','test',now() AT TIME ZONE 'UTC',2),
       ('test-archive','Test Archive','test','КАГ','каг','','test',(now() AT TIME ZONE 'UTC')-interval '2 years',1);
DO $$ BEGIN
  IF NOT EXISTS(SELECT 1 FROM studies WHERE study_id='test-current' AND payment='vmp' AND room=2) THEN
    RAISE EXCEPTION 'Current operation must match VMP rule and preserve room'; END IF;
  IF NOT EXISTS(SELECT 1 FROM studies WHERE study_id='test-archive' AND payment='' AND NOT payment_pending) THEN
    RAISE EXCEPTION 'Archive must not acquire a payment classification'; END IF;
END $$;
INSERT INTO agent_logs(agent_id,period_start,period_end,content)
VALUES (987654,now()-interval '1 hour',now(),'2026-10-03 10:00:00,000 | WARNING | agent_1 | protocols | Empty file: C:\protocols\empty.docx');
DO $$ BEGIN
  IF NOT EXISTS(SELECT 1 FROM agent_logs WHERE agent_id=987654 AND has_issues) THEN
    RAISE EXCEPTION 'Structured warning must activate alert'; END IF;
END $$;
ROLLBACK;

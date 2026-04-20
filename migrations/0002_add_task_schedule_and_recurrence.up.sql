ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	ADD COLUMN IF NOT EXISTS occurrence_on DATE,
	ADD COLUMN IF NOT EXISTS recurrence JSONB,
	ADD COLUMN IF NOT EXISTS series_root_id BIGINT;

UPDATE tasks
SET occurrence_on = scheduled_at::date
WHERE occurrence_on IS NULL;

UPDATE tasks
SET series_root_id = id
WHERE series_root_id IS NULL;

ALTER TABLE tasks
	ALTER COLUMN occurrence_on SET NOT NULL,
	ALTER COLUMN series_root_id SET NOT NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'fk_tasks_series_root'
			AND conrelid = 'tasks'::regclass
	) THEN
		ALTER TABLE tasks
			ADD CONSTRAINT fk_tasks_series_root
				FOREIGN KEY (series_root_id) REFERENCES tasks(id) ON DELETE CASCADE;
	END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tasks_series_occurrence
	ON tasks (series_root_id, occurrence_on);

CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_at ON tasks (scheduled_at);

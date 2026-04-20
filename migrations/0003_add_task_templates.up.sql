ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS is_template BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE tasks
SET is_template = TRUE
WHERE recurrence IS NOT NULL
	AND id = series_root_id;

CREATE INDEX IF NOT EXISTS idx_tasks_is_template ON tasks (is_template);

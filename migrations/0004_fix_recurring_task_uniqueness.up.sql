DROP INDEX IF EXISTS uq_tasks_series_occurrence;

CREATE UNIQUE INDEX IF NOT EXISTS uq_tasks_series_occurrence
	ON tasks (series_root_id, occurrence_on)
	WHERE is_template = FALSE;

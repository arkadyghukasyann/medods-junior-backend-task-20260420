package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	recurrencePayload, err := marshalRecurrence(task.Recurrence)
	if err != nil {
		return nil, err
	}

	const query = `
		WITH next_id AS (
			SELECT nextval(pg_get_serial_sequence('tasks', 'id')) AS id
		)
		INSERT INTO tasks (
			id,
			series_root_id,
			is_template,
			title,
			description,
			status,
			scheduled_at,
			occurrence_on,
			recurrence,
			created_at,
			updated_at
		)
		SELECT id, id, $1, $2, $3, $4, $5, $6, $7, $8, $9
		FROM next_id
		RETURNING id, series_root_id, is_template, title, description, status, scheduled_at, recurrence, created_at, updated_at
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.IsTemplate,
		task.Title,
		task.Description,
		task.Status,
		task.ScheduledAt,
		task.ScheduledAt.Format("2006-01-02"),
		recurrencePayload,
		task.CreatedAt,
		task.UpdatedAt,
	)

	return scanTask(row)
}

func (r *Repository) CreateOccurrences(ctx context.Context, tasks []taskdomain.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for i := range tasks {
		task := tasks[i]
		batch.Queue(`
			INSERT INTO tasks (
				series_root_id,
				is_template,
				title,
				description,
				status,
				scheduled_at,
				occurrence_on,
				created_at,
				updated_at
			)
			VALUES ($1, FALSE, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (series_root_id, occurrence_on) DO NOTHING
		`,
			task.SeriesRootID,
			task.Title,
			task.Description,
			task.Status,
			task.ScheduledAt,
			task.ScheduledAt.Format("2006-01-02"),
			task.CreatedAt,
			task.UpdatedAt,
		)
	}

	results := r.pool.SendBatch(ctx, batch)
	defer results.Close()

	for range tasks {
		if _, err := results.Exec(); err != nil {
			return err
		}
	}

	return nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, series_root_id, is_template, title, description, status, scheduled_at, recurrence, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	recurrencePayload, err := marshalRecurrence(task.Recurrence)
	if err != nil {
		return nil, err
	}

	const query = `
		UPDATE tasks
		SET is_template = $1,
			title = $2,
			description = $3,
			status = $4,
			scheduled_at = $5,
			occurrence_on = $6,
			recurrence = $7,
			updated_at = $8
		WHERE id = $9
		RETURNING id, series_root_id, is_template, title, description, status, scheduled_at, recurrence, created_at, updated_at
	`

	row := r.pool.QueryRow(
		ctx,
		query,
		task.IsTemplate,
		task.Title,
		task.Description,
		task.Status,
		task.ScheduledAt,
		task.ScheduledAt.Format("2006-01-02"),
		recurrencePayload,
		task.UpdatedAt,
		task.ID,
	)

	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) ListTemplates(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, series_root_id, is_template, title, description, status, scheduled_at, recurrence, created_at, updated_at
		FROM tasks
		WHERE is_template = TRUE
		ORDER BY id ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

func (r *Repository) DeleteFutureOccurrences(ctx context.Context, seriesRootID int64, from time.Time) error {
	const query = `
		DELETE FROM tasks
		WHERE series_root_id = $1
			AND id <> $1
			AND is_template = FALSE
			AND scheduled_at >= $2
	`

	_, err := r.pool.Exec(ctx, query, seriesRootID, from)
	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) List(ctx context.Context, options taskusecase.ListOptions) ([]taskdomain.Task, error) {
	query := `
		SELECT id, series_root_id, is_template, title, description, status, scheduled_at, recurrence, created_at, updated_at
		FROM tasks
	`
	if !options.IncludeTemplates {
		query += ` WHERE is_template = FALSE`
	}
	query += ` ORDER BY scheduled_at ASC, id ASC`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanTasks(rows)
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTasks(rows pgx.Rows) ([]taskdomain.Task, error) {
	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task              taskdomain.Task
		status            string
		recurrencePayload []byte
	)

	if err := scanner.Scan(
		&task.ID,
		&task.SeriesRootID,
		&task.IsTemplate,
		&task.Title,
		&task.Description,
		&status,
		&task.ScheduledAt,
		&recurrencePayload,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)
	if len(recurrencePayload) > 0 {
		var recurrence taskdomain.Recurrence
		if err := json.Unmarshal(recurrencePayload, &recurrence); err != nil {
			return nil, err
		}

		task.Recurrence = &recurrence
	}

	return &task, nil
}

func marshalRecurrence(recurrence *taskdomain.Recurrence) ([]byte, error) {
	if recurrence == nil {
		return nil, nil
	}

	return json.Marshal(recurrence)
}

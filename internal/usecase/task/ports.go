package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	CreateOccurrences(ctx context.Context, tasks []taskdomain.Task) error
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	ListTemplates(ctx context.Context) ([]taskdomain.Task, error)
	DeleteFutureOccurrences(ctx context.Context, seriesRootID int64, from time.Time) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, options ListOptions) ([]taskdomain.Task, error)
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, options ListOptions) ([]taskdomain.Task, error)
}

type ListOptions struct {
	IncludeTemplates bool
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	ScheduledAt time.Time
	Recurrence  *taskdomain.Recurrence
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	ScheduledAt time.Time
	Recurrence  *taskdomain.Recurrence
}

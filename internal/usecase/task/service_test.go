package task

import (
	"context"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type stubRepository struct {
	nextID             int64
	tasks              map[int64]*taskdomain.Task
	createdOccurrences []taskdomain.Task
	deleteFutureCalls  int
}

func newStubRepository() *stubRepository {
	return &stubRepository{
		nextID: 1,
		tasks:  make(map[int64]*taskdomain.Task),
	}
}

func (r *stubRepository) Create(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	clone := *task
	clone.ID = r.nextID
	clone.SeriesRootID = clone.ID
	r.tasks[clone.ID] = &clone
	r.nextID++

	return &clone, nil
}

func (r *stubRepository) CreateOccurrences(_ context.Context, tasks []taskdomain.Task) error {
	for i := range tasks {
		clone := tasks[i]
		clone.ID = r.nextID
		r.nextID++
		r.tasks[clone.ID] = &clone
		r.createdOccurrences = append(r.createdOccurrences, clone)
	}

	return nil
}

func (r *stubRepository) GetByID(_ context.Context, id int64) (*taskdomain.Task, error) {
	task, ok := r.tasks[id]
	if !ok {
		return nil, taskdomain.ErrNotFound
	}

	clone := *task
	return &clone, nil
}

func (r *stubRepository) Update(_ context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	clone := *task
	r.tasks[clone.ID] = &clone
	return &clone, nil
}

func (r *stubRepository) ListTemplates(_ context.Context) ([]taskdomain.Task, error) {
	result := make([]taskdomain.Task, 0)
	for _, task := range r.tasks {
		if task.IsTemplate {
			result = append(result, *task)
		}
	}

	return result, nil
}

func (r *stubRepository) DeleteFutureOccurrences(_ context.Context, seriesRootID int64, from time.Time) error {
	r.deleteFutureCalls++

	for id, task := range r.tasks {
		if task.SeriesRootID == seriesRootID && task.ID != seriesRootID && !task.ScheduledAt.Before(from) {
			delete(r.tasks, id)
		}
	}

	return nil
}

func (r *stubRepository) Delete(_ context.Context, _ int64) error {
	return nil
}

func (r *stubRepository) List(_ context.Context, options ListOptions) ([]taskdomain.Task, error) {
	result := make([]taskdomain.Task, 0)
	for _, task := range r.tasks {
		if !options.IncludeTemplates && task.IsTemplate {
			continue
		}

		result = append(result, *task)
	}

	return result, nil
}

func TestCreateRecurringTemplateMaterializesOnlyWindowOccurrences(t *testing.T) {
	repo := newStubRepository()
	service := NewService(repo)
	service.now = func() time.Time {
		return time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	}

	task, err := service.Create(context.Background(), CreateInput{
		Title:       "Call patients",
		Description: "Daily follow-up",
		Status:      taskdomain.StatusNew,
		ScheduledAt: time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:  taskdomain.RecurrenceSpecificDates,
			Dates: []string{"2026-04-24", "2026-04-30"},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if !task.IsTemplate {
		t.Fatal("expected recurring task to be stored as template")
	}

	if len(repo.createdOccurrences) != 3 {
		t.Fatalf("expected 3 generated occurrences in visible window, got %d", len(repo.createdOccurrences))
	}

	if got := repo.createdOccurrences[0].ScheduledAt.Format(time.RFC3339); got != "2026-04-21T10:00:00Z" {
		t.Fatalf("unexpected first generated occurrence: %s", got)
	}
}

func TestCreateRecurringTemplatePropagatesStatusToOccurrences(t *testing.T) {
	repo := newStubRepository()
	service := NewService(repo)
	service.now = func() time.Time {
		return time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	}

	_, err := service.Create(context.Background(), CreateInput{
		Title:       "Call patients",
		Description: "Daily follow-up",
		Status:      taskdomain.StatusInProgress,
		ScheduledAt: time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 2,
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if len(repo.createdOccurrences) == 0 {
		t.Fatal("expected generated occurrences")
	}

	for _, occurrence := range repo.createdOccurrences {
		if occurrence.Status != taskdomain.StatusInProgress {
			t.Fatalf("expected propagated status %q, got %q", taskdomain.StatusInProgress, occurrence.Status)
		}
	}
}

func TestUpdateRejectsRecurrenceChangeForGeneratedOccurrence(t *testing.T) {
	repo := newStubRepository()
	repo.tasks[10] = &taskdomain.Task{
		ID:           10,
		SeriesRootID: 5,
		Title:        "Inventory",
		Description:  "Weekly stock check",
		Status:       taskdomain.StatusNew,
		ScheduledAt:  time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
	}

	service := NewService(repo)

	_, err := service.Update(context.Background(), 10, UpdateInput{
		Title:       "Inventory",
		Description: "Weekly stock check",
		Status:      taskdomain.StatusDone,
		ScheduledAt: time.Date(2026, 4, 22, 9, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 1,
		},
	})
	if err == nil {
		t.Fatal("expected recurrence update error, got nil")
	}
}

func TestListHidesTemplatesAndSyncsOccurrences(t *testing.T) {
	repo := newStubRepository()
	repo.tasks[7] = &taskdomain.Task{
		ID:           7,
		SeriesRootID: 7,
		IsTemplate:   true,
		Title:        "Ward rounds",
		Description:  "Morning checks",
		Status:       taskdomain.StatusNew,
		ScheduledAt:  time.Date(2026, 4, 21, 8, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 2,
		},
	}

	service := NewService(repo)
	service.now = func() time.Time {
		return time.Date(2026, 4, 21, 7, 0, 0, 0, time.UTC)
	}

	tasks, err := service.List(context.Background(), ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(repo.createdOccurrences) == 0 {
		t.Fatal("expected template sync to materialize visible occurrences")
	}

	for _, task := range tasks {
		if task.IsTemplate {
			t.Fatal("expected default list to hide template rows")
		}
	}
}

func TestUpdateRecurringTemplateRefreshesFutureOccurrences(t *testing.T) {
	repo := newStubRepository()
	repo.tasks[7] = &taskdomain.Task{
		ID:           7,
		SeriesRootID: 7,
		IsTemplate:   true,
		Title:        "Ward rounds",
		Description:  "Morning checks",
		Status:       taskdomain.StatusNew,
		ScheduledAt:  time.Date(2026, 4, 21, 8, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 2,
		},
	}

	service := NewService(repo)
	service.now = func() time.Time {
		return time.Date(2026, 4, 21, 7, 0, 0, 0, time.UTC)
	}

	_, err := service.Update(context.Background(), 7, UpdateInput{
		Title:       "Ward rounds updated",
		Description: "Morning checks",
		Status:      taskdomain.StatusInProgress,
		ScheduledAt: time.Date(2026, 4, 21, 8, 0, 0, 0, time.UTC),
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 3,
		},
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if repo.deleteFutureCalls != 1 {
		t.Fatalf("expected future occurrences cleanup to be called once, got %d", repo.deleteFutureCalls)
	}

	if len(repo.createdOccurrences) == 0 {
		t.Fatal("expected regenerated occurrences after recurring template update")
	}
}

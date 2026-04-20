package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

const occurrenceLookaheadDays = 30

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	now := s.now()
	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		ScheduledAt: normalized.ScheduledAt,
		Recurrence:  normalized.Recurrence,
		IsTemplate:  normalized.Recurrence != nil,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	if err := s.syncTemplateOccurrences(ctx, created); err != nil {
		return nil, err
	}

	return created, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	found, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.syncTemplateOccurrences(ctx, found); err != nil {
		return nil, err
	}

	return found, nil
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	current, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if current.SeriesRootID != current.ID && normalized.Recurrence != nil {
		return nil, fmt.Errorf("%w", ErrRecurrenceUpdate)
	}

	updatedModel := &taskdomain.Task{
		ID:           current.ID,
		SeriesRootID: current.SeriesRootID,
		IsTemplate:   normalized.Recurrence != nil && current.SeriesRootID == current.ID,
		Title:        normalized.Title,
		Description:  normalized.Description,
		Status:       normalized.Status,
		ScheduledAt:  normalized.ScheduledAt,
		Recurrence:   normalized.Recurrence,
		CreatedAt:    current.CreatedAt,
		UpdatedAt:    s.now(),
	}

	updated, err := s.repo.Update(ctx, updatedModel)
	if err != nil {
		return nil, err
	}

	if current.SeriesRootID == current.ID {
		if current.IsTemplate || updated.IsTemplate {
			if err := s.repo.DeleteFutureOccurrences(ctx, updated.SeriesRootID, s.now()); err != nil {
				return nil, err
			}
		}

		if err := s.syncTemplateOccurrences(ctx, updated); err != nil {
			return nil, err
		}
	}

	return updated, nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) List(ctx context.Context, options ListOptions) ([]taskdomain.Task, error) {
	templates, err := s.repo.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}

	for i := range templates {
		if err := s.syncTemplateOccurrences(ctx, &templates[i]); err != nil {
			return nil, err
		}
	}

	return s.repo.List(ctx, options)
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.ScheduledAt.IsZero() {
		return CreateInput{}, fmt.Errorf("%w: scheduled_at is required", ErrInvalidInput)
	}

	recurrence, err := input.Recurrence.Normalize(input.ScheduledAt)
	if err != nil {
		return CreateInput{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	input.Recurrence = recurrence

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.ScheduledAt.IsZero() {
		return UpdateInput{}, fmt.Errorf("%w: scheduled_at is required", ErrInvalidInput)
	}

	recurrence, err := input.Recurrence.Normalize(input.ScheduledAt)
	if err != nil {
		return UpdateInput{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	input.Recurrence = recurrence

	return input, nil
}

func (s *Service) syncTemplateOccurrences(ctx context.Context, task *taskdomain.Task) error {
	if task == nil || !task.IsTemplate || task.Recurrence == nil {
		return nil
	}

	windowStart := dayStartInLocation(s.now(), task.ScheduledAt.Location())
	if task.ScheduledAt.After(windowStart) {
		windowStart = task.ScheduledAt
	}

	windowEnd := windowStart.AddDate(0, 0, occurrenceLookaheadDays)
	occurrences, err := task.Recurrence.OccurrencesInWindow(task.ScheduledAt, windowStart, windowEnd)
	if err != nil {
		return fmt.Errorf("calculate recurring occurrences: %w", err)
	}

	if len(occurrences) == 0 {
		return nil
	}

	now := s.now()
	items := make([]taskdomain.Task, 0, len(occurrences))
	for _, occurrence := range occurrences {
		items = append(items, taskdomain.Task{
			SeriesRootID: task.ID,
			Title:        task.Title,
			Description:  task.Description,
			Status:       task.Status,
			ScheduledAt:  occurrence,
			IsTemplate:   false,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	if err := s.repo.CreateOccurrences(ctx, items); err != nil {
		return fmt.Errorf("create recurring task occurrences: %w", err)
	}

	return nil
}

func dayStartInLocation(base time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}

	localized := base.In(loc)
	return time.Date(localized.Year(), localized.Month(), localized.Day(), 0, 0, 0, 0, loc)
}

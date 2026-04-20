package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type taskMutationDTO struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	ScheduledAt time.Time         `json:"scheduled_at"`
	Recurrence  *recurrenceDTO    `json:"recurrence,omitempty"`
}

type taskDTO struct {
	ID           int64             `json:"id"`
	IsTemplate   bool              `json:"is_template"`
	SeriesRootID *int64            `json:"series_root_id,omitempty"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	Status       taskdomain.Status `json:"status"`
	ScheduledAt  time.Time         `json:"scheduled_at"`
	Recurrence   *recurrenceDTO    `json:"recurrence,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type recurrenceDTO struct {
	Type       taskdomain.RecurrenceType `json:"type"`
	EveryNDays int                       `json:"every_n_days,omitempty"`
	DayOfMonth int                       `json:"day_of_month,omitempty"`
	Dates      []string                  `json:"dates,omitempty"`
}

func (dto *recurrenceDTO) toDomain() *taskdomain.Recurrence {
	if dto == nil {
		return nil
	}

	return &taskdomain.Recurrence{
		Type:       dto.Type,
		EveryNDays: dto.EveryNDays,
		DayOfMonth: dto.DayOfMonth,
		Dates:      dto.Dates,
	}
}

func newRecurrenceDTO(recurrence *taskdomain.Recurrence) *recurrenceDTO {
	if recurrence == nil {
		return nil
	}

	return &recurrenceDTO{
		Type:       recurrence.Type,
		EveryNDays: recurrence.EveryNDays,
		DayOfMonth: recurrence.DayOfMonth,
		Dates:      recurrence.Dates,
	}
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	var seriesRootID *int64
	if task.IsTemplate || task.SeriesRootID != task.ID {
		seriesRootID = &task.SeriesRootID
	}

	return taskDTO{
		ID:           task.ID,
		IsTemplate:   task.IsTemplate,
		SeriesRootID: seriesRootID,
		Title:        task.Title,
		Description:  task.Description,
		Status:       task.Status,
		ScheduledAt:  task.ScheduledAt,
		Recurrence:   newRecurrenceDTO(task.Recurrence),
		CreatedAt:    task.CreatedAt,
		UpdatedAt:    task.UpdatedAt,
	}
}

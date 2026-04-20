package task

import "time"

type Status string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID           int64       `json:"id"`
	SeriesRootID int64       `json:"series_root_id"`
	IsTemplate   bool        `json:"is_template"`
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	Status       Status      `json:"status"`
	ScheduledAt  time.Time   `json:"scheduled_at"`
	Recurrence   *Recurrence `json:"recurrence,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

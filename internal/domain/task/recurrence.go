package task

import (
	"fmt"
	"slices"
	"time"
)

const dateLayout = "2006-01-02"

type RecurrenceType string

const (
	RecurrenceDaily         RecurrenceType = "daily"
	RecurrenceMonthly       RecurrenceType = "monthly"
	RecurrenceSpecificDates RecurrenceType = "specific_dates"
	RecurrenceOddDays       RecurrenceType = "odd_days"
	RecurrenceEvenDays      RecurrenceType = "even_days"
)

type Recurrence struct {
	Type          RecurrenceType `json:"type"`
	EveryNDays    int            `json:"every_n_days,omitempty"`
	DayOfMonth    int            `json:"day_of_month,omitempty"`
	Dates         []string       `json:"dates,omitempty"`
	ExcludedDates []string       `json:"excluded_dates,omitempty"`
}

func (t RecurrenceType) Valid() bool {
	switch t {
	case RecurrenceDaily, RecurrenceMonthly, RecurrenceSpecificDates, RecurrenceOddDays, RecurrenceEvenDays:
		return true
	default:
		return false
	}
}

func (r *Recurrence) Normalize(firstOccurrence time.Time) (*Recurrence, error) {
	if r == nil {
		return nil, nil
	}

	normalized := *r
	if !normalized.Type.Valid() {
		return nil, fmt.Errorf("unsupported recurrence type")
	}

	scheduledDate := firstOccurrence.Format(dateLayout)

	switch normalized.Type {
	case RecurrenceDaily:
		if normalized.EveryNDays <= 0 {
			return nil, fmt.Errorf("every_n_days must be greater than zero")
		}

		normalized.DayOfMonth = 0
		normalized.Dates = nil
	case RecurrenceMonthly:
		if normalized.DayOfMonth < 1 || normalized.DayOfMonth > 30 {
			return nil, fmt.Errorf("day_of_month must be between 1 and 30")
		}

		if firstOccurrence.Day() != normalized.DayOfMonth {
			return nil, fmt.Errorf("scheduled_at day must match day_of_month")
		}

		normalized.EveryNDays = 0
		normalized.Dates = nil
	case RecurrenceSpecificDates:
		dates, err := normalizeDateList(append(normalized.Dates, scheduledDate))
		if err != nil {
			return nil, err
		}

		if len(dates) == 0 {
			return nil, fmt.Errorf("dates must contain at least one value")
		}

		normalized.EveryNDays = 0
		normalized.DayOfMonth = 0
		normalized.Dates = dates
	case RecurrenceOddDays:
		if firstOccurrence.Day()%2 == 0 {
			return nil, fmt.Errorf("scheduled_at must fall on an odd day")
		}

		normalized.EveryNDays = 0
		normalized.DayOfMonth = 0
		normalized.Dates = nil
	case RecurrenceEvenDays:
		if firstOccurrence.Day()%2 != 0 {
			return nil, fmt.Errorf("scheduled_at must fall on an even day")
		}

		normalized.EveryNDays = 0
		normalized.DayOfMonth = 0
		normalized.Dates = nil
	}

	excluded, err := normalizeDateList(normalized.ExcludedDates)
	if err != nil {
		return nil, err
	}

	normalized.ExcludedDates = excluded

	return &normalized, nil
}

func (r *Recurrence) OccurrencesInWindow(firstOccurrence, from, until time.Time) ([]time.Time, error) {
	if r == nil {
		return nil, nil
	}

	normalized, err := r.Normalize(firstOccurrence)
	if err != nil {
		return nil, err
	}

	if until.Before(from) {
		return nil, nil
	}

	excluded := make(map[string]struct{}, len(normalized.ExcludedDates))
	for _, value := range normalized.ExcludedDates {
		excluded[value] = struct{}{}
	}

	appendIfAllowed := func(values []time.Time, current time.Time) []time.Time {
		if current.Before(firstOccurrence) {
			return values
		}

		if current.Before(from) || current.After(until) {
			return values
		}

		if _, skip := excluded[current.Format(dateLayout)]; skip {
			return values
		}

		return append(values, current)
	}

	occurrences := make([]time.Time, 0)

	switch normalized.Type {
	case RecurrenceDaily:
		current := firstOccurrence
		if current.Before(from) {
			daysDiff := int(from.Sub(current).Hours() / 24)
			stepCount := daysDiff / normalized.EveryNDays
			current = current.AddDate(0, 0, stepCount*normalized.EveryNDays)
			for current.Before(from) {
				current = current.AddDate(0, 0, normalized.EveryNDays)
			}
		}

		for !current.After(until) {
			occurrences = appendIfAllowed(occurrences, current)
			current = current.AddDate(0, 0, normalized.EveryNDays)
		}
	case RecurrenceMonthly:
		current := firstOccurrence
		for current.Before(from) {
			current = time.Date(
				current.Year(),
				current.Month(),
				normalized.DayOfMonth,
				firstOccurrence.Hour(),
				firstOccurrence.Minute(),
				firstOccurrence.Second(),
				firstOccurrence.Nanosecond(),
				firstOccurrence.Location(),
			).AddDate(0, 1, 0)
		}

		for !current.After(until) {
			occurrences = appendIfAllowed(occurrences, current)
			current = time.Date(
				current.Year(),
				current.Month(),
				normalized.DayOfMonth,
				firstOccurrence.Hour(),
				firstOccurrence.Minute(),
				firstOccurrence.Second(),
				firstOccurrence.Nanosecond(),
				firstOccurrence.Location(),
			).AddDate(0, 1, 0)
		}
	case RecurrenceSpecificDates:
		for _, rawDate := range normalized.Dates {
			dateValue, err := time.ParseInLocation(dateLayout, rawDate, firstOccurrence.Location())
			if err != nil {
				return nil, fmt.Errorf("parse date %q: %w", rawDate, err)
			}

			current := time.Date(
				dateValue.Year(),
				dateValue.Month(),
				dateValue.Day(),
				firstOccurrence.Hour(),
				firstOccurrence.Minute(),
				firstOccurrence.Second(),
				firstOccurrence.Nanosecond(),
				firstOccurrence.Location(),
			)
			occurrences = appendIfAllowed(occurrences, current)
		}
	case RecurrenceOddDays, RecurrenceEvenDays:
		cursor := time.Date(
			from.Year(),
			from.Month(),
			from.Day(),
			firstOccurrence.Hour(),
			firstOccurrence.Minute(),
			firstOccurrence.Second(),
			firstOccurrence.Nanosecond(),
			firstOccurrence.Location(),
		)

		if cursor.Before(firstOccurrence) {
			cursor = firstOccurrence
		}

		for !cursor.After(until) {
			if normalized.Type == RecurrenceOddDays && cursor.Day()%2 != 0 {
				occurrences = appendIfAllowed(occurrences, cursor)
			}
			if normalized.Type == RecurrenceEvenDays && cursor.Day()%2 == 0 {
				occurrences = appendIfAllowed(occurrences, cursor)
			}
			cursor = cursor.AddDate(0, 0, 1)
		}
	}

	return occurrences, nil
}

func normalizeDateList(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))

	for _, value := range values {
		parsed, err := time.Parse(dateLayout, value)
		if err != nil {
			return nil, fmt.Errorf("invalid date %q", value)
		}

		formatted := parsed.Format(dateLayout)
		if _, exists := seen[formatted]; exists {
			continue
		}

		seen[formatted] = struct{}{}
		normalized = append(normalized, formatted)
	}

	slices.Sort(normalized)

	return normalized, nil
}

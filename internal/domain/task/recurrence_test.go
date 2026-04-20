package task

import (
	"testing"
	"time"
)

func TestRecurrenceNormalizeSpecificDatesAddsScheduledDate(t *testing.T) {
	recurrence := &Recurrence{
		Type:  RecurrenceSpecificDates,
		Dates: []string{"2026-04-25", "2026-04-28"},
	}

	normalized, err := recurrence.Normalize(time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	expected := []string{"2026-04-21", "2026-04-25", "2026-04-28"}
	assertStringSliceEqual(t, expected, normalized.Dates)
}

func TestRecurrenceOccurrencesInWindowDaily(t *testing.T) {
	firstOccurrence := time.Date(2026, 4, 21, 10, 30, 0, 0, time.UTC)
	recurrence := &Recurrence{
		Type:       RecurrenceDaily,
		EveryNDays: 2,
	}

	occurrences, err := recurrence.OccurrencesInWindow(
		firstOccurrence,
		time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 27, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("OccurrencesInWindow() error = %v", err)
	}

	expected := []time.Time{
		time.Date(2026, 4, 21, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 23, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 25, 10, 30, 0, 0, time.UTC),
		time.Date(2026, 4, 27, 10, 30, 0, 0, time.UTC),
	}

	assertTimeSliceEqual(t, expected, occurrences)
}

func TestRecurrenceOccurrencesInWindowMonthly(t *testing.T) {
	firstOccurrence := time.Date(2026, 4, 15, 8, 0, 0, 0, time.UTC)
	recurrence := &Recurrence{
		Type:       RecurrenceMonthly,
		DayOfMonth: 15,
	}

	occurrences, err := recurrence.OccurrencesInWindow(
		firstOccurrence,
		time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("OccurrencesInWindow() error = %v", err)
	}

	expected := []time.Time{
		time.Date(2026, 5, 15, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 15, 8, 0, 0, 0, time.UTC),
	}

	assertTimeSliceEqual(t, expected, occurrences)
}

func TestRecurrenceOccurrencesInWindowEvenDays(t *testing.T) {
	firstOccurrence := time.Date(2026, 4, 20, 7, 45, 0, 0, time.UTC)
	recurrence := &Recurrence{
		Type: RecurrenceEvenDays,
	}

	occurrences, err := recurrence.OccurrencesInWindow(
		firstOccurrence,
		time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 25, 23, 59, 59, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("OccurrencesInWindow() error = %v", err)
	}

	expected := []time.Time{
		time.Date(2026, 4, 22, 7, 45, 0, 0, time.UTC),
		time.Date(2026, 4, 24, 7, 45, 0, 0, time.UTC),
	}

	assertTimeSliceEqual(t, expected, occurrences)
}

func assertTimeSliceEqual(t *testing.T, expected, actual []time.Time) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Fatalf("expected %d values, got %d", len(expected), len(actual))
	}

	for i := range expected {
		if !expected[i].Equal(actual[i]) {
			t.Fatalf("expected %s at index %d, got %s", expected[i], i, actual[i])
		}
	}
}

func assertStringSliceEqual(t *testing.T, expected, actual []string) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Fatalf("expected %d values, got %d", len(expected), len(actual))
	}

	for i := range expected {
		if expected[i] != actual[i] {
			t.Fatalf("expected %q at index %d, got %q", expected[i], i, actual[i])
		}
	}
}

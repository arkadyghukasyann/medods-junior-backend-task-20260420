package task

import "errors"

var ErrInvalidInput = errors.New("invalid task input")
var ErrRecurrenceUpdate = errors.New("recurrence settings can only be changed on the series root task")

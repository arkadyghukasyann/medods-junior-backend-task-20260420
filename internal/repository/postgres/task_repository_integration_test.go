package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	dbinfra "example.com/taskservice/internal/infrastructure/postgres"
	taskusecase "example.com/taskservice/internal/usecase/task"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCreateOccurrencesKeepsFirstVisibleTaskWhenTemplateSharesDate(t *testing.T) {
	pool, cleanup := newTestPool(t)
	defer cleanup()

	repo := New(pool)
	now := time.Now().UTC()
	scheduledAt := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)

	template, err := repo.Create(context.Background(), &taskdomain.Task{
		Title:       "Call patients",
		Description: "Daily follow-up",
		Status:      taskdomain.StatusNew,
		ScheduledAt: scheduledAt,
		Recurrence: &taskdomain.Recurrence{
			Type:       taskdomain.RecurrenceDaily,
			EveryNDays: 7,
		},
		IsTemplate: true,
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err = repo.CreateOccurrences(context.Background(), []taskdomain.Task{
		{
			SeriesRootID: template.ID,
			Title:        template.Title,
			Description:  template.Description,
			Status:       taskdomain.StatusNew,
			ScheduledAt:  scheduledAt,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	})
	if err != nil {
		t.Fatalf("CreateOccurrences() error = %v", err)
	}

	tasks, err := repo.List(context.Background(), taskusecase.ListOptions{IncludeTemplates: true})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var visibleMatches int
	for _, task := range tasks {
		if !task.IsTemplate && task.SeriesRootID == template.ID && task.ScheduledAt.Equal(scheduledAt) {
			visibleMatches++
		}
	}

	if visibleMatches != 1 {
		t.Fatalf("expected exactly one visible occurrence for the template date, got %d", visibleMatches)
	}
}

func newTestPool(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	dsn := os.Getenv("TASK_SERVICE_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TASK_SERVICE_TEST_DATABASE_DSN is not set")
	}

	ctx := context.Background()

	adminConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("ParseConfig(admin) error = %v", err)
	}
	adminConfig.ConnConfig.Database = "postgres"

	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("NewWithConfig(admin) error = %v", err)
	}

	dbName := fmt.Sprintf("taskservice_test_%d", time.Now().UnixNano())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		adminPool.Close()
		t.Fatalf("create database error = %v", err)
	}

	testConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		adminPool.Close()
		t.Fatalf("ParseConfig(test) error = %v", err)
	}
	testConfig.ConnConfig.Database = dbName

	testPool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		_, _ = adminPool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, dbName))
		adminPool.Close()
		t.Fatalf("NewWithConfig(test) error = %v", err)
	}

	if err := dbinfra.ApplyMigrations(ctx, testPool); err != nil {
		testPool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, dbName))
		adminPool.Close()
		t.Fatalf("ApplyMigrations() error = %v", err)
	}

	cleanup := func() {
		testPool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s" WITH (FORCE)`, dbName))
		adminPool.Close()
	}

	return testPool, cleanup
}

package app

import (
	"context"
	"fmt"
	"net/http"
	"os"

	infrastructurepostgres "example.com/taskservice/internal/infrastructure/postgres"
	postgresrepo "example.com/taskservice/internal/repository/postgres"
	transporthttp "example.com/taskservice/internal/transport/http"
	swaggerdocs "example.com/taskservice/internal/transport/http/docs"
	httphandlers "example.com/taskservice/internal/transport/http/handlers"
	"example.com/taskservice/internal/usecase/task"
)

type Config struct {
	HTTPAddr    string
	DatabaseDSN string
}

type Runtime struct {
	Router http.Handler
	close  func()
}

func (r *Runtime) Close() {
	if r != nil && r.close != nil {
		r.close()
	}
}

func LoadConfig() Config {
	cfg := Config{
		HTTPAddr:    envOrDefault("HTTP_ADDR", ":8080"),
		DatabaseDSN: firstNonEmpty(os.Getenv("DATABASE_DSN"), os.Getenv("NETLIFY_DATABASE_URL"), "postgres://postgres:postgres@localhost:5432/taskservice?sslmode=disable"),
	}

	if cfg.DatabaseDSN == "" {
		panic(fmt.Errorf("database dsn is required"))
	}

	return cfg
}

func NewRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	pool, err := infrastructurepostgres.Open(ctx, cfg.DatabaseDSN)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := infrastructurepostgres.ApplyMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	taskRepo := postgresrepo.New(pool)
	taskUsecase := task.NewService(taskRepo)
	taskHandler := httphandlers.NewTaskHandler(taskUsecase)
	docsHandler := swaggerdocs.NewHandler()
	router := transporthttp.NewRouter(taskHandler, docsHandler)

	return &Runtime{
		Router: router,
		close: func() {
			pool.Close()
		},
	}, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}

	return ""
}

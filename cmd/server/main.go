package main

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/clients/adzuna"
	"nice/internal/config"
	"nice/internal/migrations"
	"nice/internal/repositories"
	"nice/internal/server"
	"nice/internal/services"
)

func main() {
	fx.New(
		fx.Provide(
			newLogger,
			config.Load,
			openDatabase,
			adzuna.NewClient,
			repositories.NewSQLite,
			services.NewJobSync,
			server.New,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	db *sql.DB,
	logger *zap.Logger,
	server *server.Server,
	sync *services.JobSync,
) {
	lifecycle.Append(fx.Hook{
		OnStop: func(context.Context) error {
			err := db.Close()
			_ = logger.Sync()
			return err
		},
	})
	server.Register(lifecycle)
	sync.Register(lifecycle)
}

func newLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stderr", "jobs.log"}
	config.ErrorOutputPaths = config.OutputPaths
	return config.Build()
}

func openDatabase(cfg config.Config, logger *zap.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.Database.Path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := migrations.Apply(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	logger.Info("database migrations applied", zap.String("path", cfg.Database.Path))
	return db, nil
}

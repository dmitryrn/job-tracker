package main

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/openrouter"
	"nice/internal/clients/remotive"
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
			jobicy.NewClient,
			openrouter.NewClient,
			remotive.NewClient,
			newJobCompletionClient,
			repositories.NewSQLite,
			services.NewJobSync,
			services.NewJobBrowse,
			services.NewUserProfileService,
			newJobAnalyzer,
			newJobAnalysisService,
			newProfileJobMatcher,
			services.NewJobMatchProcessor,
			services.NewJobMatches,
			services.NewJobMatchRequests,
			server.New,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func newJobCompletionClient(client *openrouter.Client) services.JobCompletionClient {
	return client
}

func newJobAnalyzer(client services.JobCompletionClient, cfg config.Config) *services.JobAnalyzer {
	return services.NewJobAnalyzer(client, cfg.LLM.JobAnalysis.Model, cfg.LLM.JobAnalysis.ReasoningEffort)
}

func newJobAnalysisService(analyzer *services.JobAnalyzer) services.JobAnalysisService {
	return analyzer
}

func newProfileJobMatcher(client services.JobCompletionClient, cfg config.Config) services.ProfileJobMatcher {
	return services.NewLLMProfileJobMatcher(client, cfg.LLM.ProfileMatcher.Model, cfg.LLM.ProfileMatcher.ReasoningEffort)
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	db *sql.DB,
	logger *zap.Logger,
	server *server.Server,
	sync *services.JobSync,
	matchProcessor *services.JobMatchProcessor,
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
	matchProcessor.Register(lifecycle)
}

func newLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	return config.Build()
}

func openDatabase(cfg config.Config, logger *zap.Logger) (*sql.DB, error) {
	db, err := sql.Open("sqlite", cfg.DatabasePath)
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
	logger.Info("database migrations applied", zap.String("path", cfg.DatabasePath))
	return db, nil
}

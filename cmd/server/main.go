package main

import (
	"context"
	"database/sql"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"
	_ "modernc.org/sqlite"

	"nice/internal/clients/adzuna"
	"nice/internal/clients/ipinfo"
	"nice/internal/clients/jobicy"
	"nice/internal/clients/linkedin"
	"nice/internal/clients/openrouter"
	"nice/internal/clients/remotive"
	"nice/internal/config"
	"nice/internal/migrations"
	"nice/internal/models"
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
			ipinfo.NewClient,
			jobicy.NewClient,
			linkedin.NewClient,
			openrouter.NewClient,
			remotive.NewClient,
			newJobCompletionClient,
			repositories.NewSQLite,
			repositories.NewJobRepository,
			repositories.NewProviderRunRepository,
			repositories.NewEventRepository,
			services.NewEventWriter,
			services.NewEventRecorder,
			repositories.NewDiscoverySettingsRepository,
			repositories.NewUserProfileRepository,
			repositories.NewJobAnalysisRepository,
			repositories.NewJobMatchRepository,
			repositories.NewMatchQueueRepository,
			services.NewJobSync,
			services.NewLinkedInJobs,
			services.NewJobBrowse,
			services.NewEventLog,
			services.NewDiscoverySettingsService,
			services.NewProviderPreviewService,
			services.NewUserProfileService,
			newJobAnalyzer,
			newJobAnalysisService,
			newProfileJobMatcher,
			newJobMatchWorker,
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

func newJobMatchWorker(jobs repositories.JobRepository, analyses repositories.JobAnalysisRepository, matches repositories.JobMatchRepository, queue repositories.MatchQueueRepository, profiles repositories.UserProfileRepository, analyzer services.JobAnalysisService, matcher services.ProfileJobMatcher, logger *zap.Logger, cfg config.Config) *services.JobMatchWorker {
	return services.NewJobMatchWorker(jobs, analyses, matches, queue, profiles, analyzer, matcher, logger, cfg.JobMatch.RunInterval)
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	db *sql.DB,
	logger *zap.Logger,
	eventWriter *services.EventWriter,
	server *server.Server,
	sync *services.JobSync,
	matchWorker *services.JobMatchWorker,
) {
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			err := db.Close()
			_ = logger.Sync()
			return err
		},
	})
	eventWriter.Register(lifecycle)
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			if err := eventWriter.RecordEvent(ctx, models.Event{Provider: "application", Type: "application.started", Level: "info", Message: "Application started"}); err != nil {
				logger.Error("record application start event failed", zap.Error(err))
			}
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if eventErr := eventWriter.RecordEvent(ctx, models.Event{Provider: "application", Type: "application.stopped", Level: "info", Message: "Application stopped"}); eventErr != nil {
				logger.Error("record application stop event failed", zap.Error(eventErr))
			}
			return nil
		},
	})
	server.Register(lifecycle)
	sync.Register(lifecycle)
	matchWorker.Register(lifecycle)
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

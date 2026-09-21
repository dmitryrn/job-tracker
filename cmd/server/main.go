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
	"nice/internal/clients/openai"
	"nice/internal/clients/remotive"
	"nice/internal/clients/typesafe"
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
			fx.Annotate(newOpenCodeClient, fx.ResultTags(`name:"opencode"`)),
			fx.Annotate(newOpenAIClient, fx.ResultTags(`name:"openai"`)),
			newTypeSafeClient,
			remotive.NewClient,
			fx.Annotate(newCompletionClients, fx.ParamTags(`name:"opencode"`, `name:"openai"`)),
			repositories.NewSQLite,
			repositories.NewJobRepository,
			repositories.NewProviderRunRepository,
			repositories.NewEventRepository,
			services.NewEventWriter,
			services.NewEventRecorder,
			repositories.NewDiscoverySettingsRepository,
			repositories.NewUserProfileRepository,
			repositories.NewResumeRepository,
			repositories.NewJobAnalysisRepository,
			repositories.NewJobMatchRepository,
			repositories.NewApplicationRepository,
			repositories.NewJobMatchChatRepository,
			repositories.NewMatchQueueRepository,
			services.NewLinkedInMetrics,
			services.NewJobSync,
			services.NewLinkedInJobs,
			fx.Annotate(newCustomJobImporter, fx.As(new(services.CustomJobImportService))),
			services.NewJobBrowse,
			services.NewEventLog,
			services.NewDiscoverySettingsService,
			services.NewProviderPreviewService,
			services.NewUserProfileService,
			services.NewResumeService,
			services.NewResumePDFService,
			newJobAnalyzer,
			newJobProfileScorer,
			newJobAnalysisService,
			newProfileJobMatcher,
			newJobMatchWorker,
			services.NewJobMatches,
			services.NewApplications,
			services.NewJobMatchRequests,
			newJobMatchChat,
			server.New,
		),
		fx.Invoke(registerLifecycle),
	).Run()
}

func newOpenCodeClient(cfg config.Config) *openai.Client {
	return openai.NewClient(cfg.OpenCode.APIKey, cfg.OpenCode.BaseURL)
}

func newOpenAIClient(cfg config.Config) *openai.Client {
	return openai.NewClient(cfg.OpenAI.APIKey, cfg.OpenAI.BaseURL)
}

func newTypeSafeClient(cfg config.Config) (*typesafe.Client, error) {
	return typesafe.NewClient(cfg.TypeSafe.APIKey, cfg.TypeSafe.BaseURL, cfg.TypeSafe.Model)
}

type completionClients struct {
	openCode *openai.Client
	openAI   *openai.Client
}

func newCompletionClients(openCode, openAI *openai.Client) completionClients {
	return completionClients{openCode: openCode, openAI: openAI}
}

func (clients completionClients) forProvider(provider string) (services.JobCompletionClient, error) {
	switch provider {
	case "opencode":
		return clients.openCode, nil
	case "openai":
		return clients.openAI, nil
	default:
		return nil, fmt.Errorf("unsupported completion provider %q", provider)
	}
}

func newJobAnalyzer(clients completionClients, cfg config.Config) (*services.JobAnalyzer, error) {
	client, err := clients.forProvider(cfg.JobAnalysis.Provider)
	if err != nil {
		return nil, fmt.Errorf("select job analysis client: %w", err)
	}

	return services.NewJobAnalyzer(client, cfg.JobAnalysis.Model, cfg.JobAnalysis.ReasoningEffort), nil
}

func newJobProfileScorer(client *typesafe.Client) services.JobProfileScoreService {
	return services.NewJobProfileScorer(client)
}

func newCustomJobImporter(clients completionClients, cfg config.Config) (*services.CustomJobImporter, error) {
	client, err := clients.forProvider(cfg.CustomJobImport.Provider)
	if err != nil {
		return nil, fmt.Errorf("select custom job import client: %w", err)
	}

	return services.NewCustomJobImporter(services.NewHTTPJobPageFetcher(), client, cfg.CustomJobImport.Model, cfg.CustomJobImport.ReasoningEffort), nil
}

func newJobAnalysisService(analyzer *services.JobAnalyzer) services.JobAnalysisService {
	return analyzer
}

func newProfileJobMatcher(clients completionClients, cfg config.Config) (services.ProfileJobMatcher, error) {
	client, err := clients.forProvider(cfg.ProfileMatcher.Provider)
	if err != nil {
		return nil, fmt.Errorf("select profile matcher client: %w", err)
	}

	return services.NewLLMProfileJobMatcher(client, cfg.ProfileMatcher.Model, cfg.ProfileMatcher.ReasoningEffort), nil
}

func newJobMatchChat(jobs repositories.JobRepository, matches repositories.JobMatchRepository, items repositories.JobMatchChatRepository, profiles repositories.UserProfileRepository, resumes repositories.ResumeRepository, clients completionClients, cfg config.Config, logger *zap.Logger) (*services.JobMatchChat, error) {
	client, err := clients.forProvider(cfg.JobChat.Provider)
	if err != nil {
		return nil, fmt.Errorf("select job chat client: %w", err)
	}

	return services.NewJobMatchChat(jobs, matches, items, profiles, resumes, client, cfg.JobChat.Model, cfg.JobChat.ReasoningEffort, logger), nil
}

func newJobMatchWorker(jobs repositories.JobRepository, analyses repositories.JobAnalysisRepository, matches repositories.JobMatchRepository, queue repositories.MatchQueueRepository, profiles repositories.UserProfileRepository, analyzer services.JobAnalysisService, scorer services.JobProfileScoreService, matcher services.ProfileJobMatcher, events repositories.EventRecorder, logger *zap.Logger, cfg config.Config) *services.JobMatchWorker {
	return services.NewJobMatchWorker(jobs, analyses, matches, queue, profiles, analyzer, matcher, events, logger, cfg.JobMatch.RunInterval, scorer)
}

func registerLifecycle(
	lifecycle fx.Lifecycle,
	db *sql.DB,
	logger *zap.Logger,
	eventWriter *services.EventWriter,
	server *server.Server,
	sync *services.JobSync,
	matchWorker *services.JobMatchWorker,
	chat *services.JobMatchChat,
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
	chat.Register(lifecycle)
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

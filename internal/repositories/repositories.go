package repositories

import (
	"context"
	"time"

	"nice/internal/models"
)

type JobRepository interface {
	Upsert(context.Context, []models.Job) error
	List(context.Context, models.JobSearch) ([]models.BrowseJob, error)
	Job(context.Context, int64) (*models.BrowseJob, error)
	Delete(context.Context, int64) (bool, error)
	Providers(context.Context) ([]string, error)
	Companies(context.Context, string) ([]models.BrowseCompany, error)
	AnalysisJob(context.Context, int64) (*models.Job, error)
}

type ProviderRunRepository interface {
	StartProviderRun(context.Context, string, time.Duration, time.Time) (bool, error)
}

type EventRepository interface {
	RecordEvent(context.Context, models.Event) error
	Events(context.Context, models.EventSearch) (models.EventPage, error)
}

type DiscoverySettingsRepository interface {
	DiscoverySettings(context.Context) (models.DiscoverySettings, error)
	SaveDiscoverySettings(context.Context, models.DiscoverySettings) (models.DiscoverySettings, error)
}

type UserProfileRepository interface {
	UserProfile(context.Context) (*models.UserProfile, error)
	SaveUserProfile(context.Context, models.UserProfile) (models.UserProfile, error)
}

type JobAnalysisRepository interface {
	JobAnalysis(context.Context, int64) (*models.JobAnalysisRecord, error)
	SaveJobAnalysis(context.Context, models.JobAnalysisRecord) error
}

type JobMatchRepository interface {
	JobMatchExists(context.Context, int64) (bool, error)
	CreateJobMatch(context.Context, int64, string) error
	JobMatch(context.Context, int64) (*models.JobMatchRecord, error)
	JobMatches(context.Context) ([]models.JobMatchSummary, error)
}

type MatchQueueRepository interface {
	MatchQueue(context.Context) ([]models.BrowseJob, error)
	QueueJobMatch(context.Context, int64, bool) (bool, error)
	QueueJobsWithoutMatches(context.Context, []int64) (int, error)
	ReplaceMatchQueue(context.Context, []int64) (bool, error)
	RemoveMatchRequest(context.Context, int64) error
	CompleteMatchRequest(context.Context, int64, string) error
}

func NewJobRepository(sqlite *SQLite) JobRepository {
	return sqlite
}

func NewProviderRunRepository(sqlite *SQLite) ProviderRunRepository {
	return sqlite
}

func NewEventRepository(sqlite *SQLite) EventRepository {
	return sqlite
}

func NewDiscoverySettingsRepository(sqlite *SQLite) DiscoverySettingsRepository {
	return sqlite
}

func NewUserProfileRepository(sqlite *SQLite) UserProfileRepository {
	return sqlite
}

func NewJobAnalysisRepository(sqlite *SQLite) JobAnalysisRepository {
	return sqlite
}

func NewJobMatchRepository(sqlite *SQLite) JobMatchRepository {
	return sqlite
}

func NewMatchQueueRepository(sqlite *SQLite) MatchQueueRepository {
	return sqlite
}

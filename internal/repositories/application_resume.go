package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) ApplicationResume(ctx context.Context, jobID int64) (*models.ApplicationResume, error) {
	query, arguments, err := sqlBuilder.
		Select("id", "job_id", "root_message_id", "base_resume_json", "created_at").
		From("application_resumes").
		Where(squirrel.Eq{"job_id": jobID}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get application resume query: %w", err)
	}

	var application models.ApplicationResume
	var baseJSON string
	if err := repository.db.QueryRowContext(ctx, query, arguments...).Scan(&application.ID, &application.JobID, &application.RootMessageID, &baseJSON, &application.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get application resume: %w", err)
	}
	if err := json.Unmarshal([]byte(baseJSON), &application.Base); err != nil {
		return nil, fmt.Errorf("decode application base resume: %w", err)
	}

	revisionQuery, revisionArguments, err := sqlBuilder.
		Select("id", "revision_number", "trigger_message_id", "COALESCE(assistant_message_id, 0)", "resume_json", "summary", "created_at").
		From("application_resume_revisions").
		Where(squirrel.Eq{"application_resume_id": application.ID}).
		OrderBy("revision_number").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list application resume revisions query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, revisionQuery, revisionArguments...)
	if err != nil {
		return nil, fmt.Errorf("list application resume revisions: %w", err)
	}
	defer rows.Close()
	application.Revisions = make([]models.ApplicationResumeRevision, 0)
	for rows.Next() {
		var revision models.ApplicationResumeRevision
		var resumeJSON string
		if err := rows.Scan(&revision.ID, &revision.RevisionNumber, &revision.TriggerMessageID, &revision.AssistantMessageID, &resumeJSON, &revision.Summary, &revision.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan application resume revision: %w", err)
		}
		if err := json.Unmarshal([]byte(resumeJSON), &revision.Resume); err != nil {
			return nil, fmt.Errorf("decode application resume revision: %w", err)
		}
		application.Revisions = append(application.Revisions, revision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application resume revisions: %w", err)
	}
	return &application, nil
}

func (repository *SQLite) CreateApplicationResume(ctx context.Context, jobID, rootMessageID int64, base models.Resume) (*models.ApplicationResume, bool, error) {
	baseJSON, err := json.Marshal(base)
	if err != nil {
		return nil, false, fmt.Errorf("encode application base resume: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin create application resume transaction: %w", err)
	}
	defer transaction.Rollback()

	query, arguments, err := sqlBuilder.
		Insert("application_resumes").
		Columns("job_id", "root_message_id", "base_resume_json", "created_at").
		Values(jobID, rootMessageID, string(baseJSON), now).
		Suffix("ON CONFLICT(job_id) DO NOTHING").
		ToSql()
	if err != nil {
		return nil, false, fmt.Errorf("build create application resume query: %w", err)
	}
	result, err := transaction.ExecContext(ctx, query, arguments...)
	if err != nil {
		return nil, false, fmt.Errorf("create application resume: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("check application resume creation: %w", err)
	}
	if created > 0 {
		applicationID, err := result.LastInsertId()
		if err != nil {
			return nil, false, fmt.Errorf("get application resume ID: %w", err)
		}
		revisionQuery, revisionArguments, err := sqlBuilder.
			Insert("application_resume_revisions").
			Columns("application_resume_id", "revision_number", "trigger_message_id", "resume_json", "summary", "created_at").
			Values(applicationID, 0, rootMessageID, string(baseJSON), "Snapshot of the base resume", now).
			ToSql()
		if err != nil {
			return nil, false, fmt.Errorf("build create application resume snapshot query: %w", err)
		}
		if _, err := transaction.ExecContext(ctx, revisionQuery, revisionArguments...); err != nil {
			return nil, false, fmt.Errorf("create application resume snapshot: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return nil, false, fmt.Errorf("commit create application resume: %w", err)
	}
	application, err := repository.ApplicationResume(ctx, jobID)
	if err != nil {
		return nil, false, err
	}
	return application, created > 0, nil
}

func (repository *SQLite) CreateApplicationResumeRevision(ctx context.Context, applicationID, triggerMessageID, assistantMessageID int64, resume models.Resume, summary string) (models.ApplicationResumeRevision, error) {
	resumeJSON, err := json.Marshal(resume)
	if err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("encode application resume revision: %w", err)
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("begin create application resume revision transaction: %w", err)
	}
	defer transaction.Rollback()
	var revisionNumber int
	if err := transaction.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_number), -1) + 1 FROM application_resume_revisions WHERE application_resume_id = ?`, applicationID).Scan(&revisionNumber); err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("get next application resume revision number: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	query, arguments, err := sqlBuilder.
		Insert("application_resume_revisions").
		Columns("application_resume_id", "revision_number", "trigger_message_id", "assistant_message_id", "resume_json", "summary", "created_at").
		Values(applicationID, revisionNumber, triggerMessageID, assistantMessageID, string(resumeJSON), summary, now).
		ToSql()
	if err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("build create application resume revision query: %w", err)
	}
	result, err := transaction.ExecContext(ctx, query, arguments...)
	if err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("create application resume revision: %w", err)
	}
	revisionID, err := result.LastInsertId()
	if err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("get application resume revision ID: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return models.ApplicationResumeRevision{}, fmt.Errorf("commit application resume revision: %w", err)
	}
	return models.ApplicationResumeRevision{ID: revisionID, RevisionNumber: revisionNumber, TriggerMessageID: triggerMessageID, AssistantMessageID: assistantMessageID, Resume: resume, Summary: summary, CreatedAt: now}, nil
}

func (repository *SQLite) ApplicationResumeAgentEvents(ctx context.Context, jobID int64) ([]models.ApplicationResumeAgentEvent, error) {
	query, arguments, err := sqlBuilder.
		Select("events.id", "events.trigger_message_id", "COALESCE(events.revision_id, 0)", "events.event_type", "events.detail", "events.created_at").
		From("application_resume_agent_events events").
		Join("job_match_chat_messages messages ON messages.id = events.trigger_message_id").
		Where(squirrel.Eq{"messages.job_id": jobID}).
		OrderBy("events.id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list application resume agent events query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list application resume agent events: %w", err)
	}
	defer rows.Close()
	events := make([]models.ApplicationResumeAgentEvent, 0)
	for rows.Next() {
		var event models.ApplicationResumeAgentEvent
		if err := rows.Scan(&event.ID, &event.TriggerMessageID, &event.RevisionID, &event.Type, &event.Detail, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan application resume agent event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate application resume agent events: %w", err)
	}
	return events, nil
}

func (repository *SQLite) CreateApplicationResumeAgentEvent(ctx context.Context, event models.ApplicationResumeAgentEvent) (models.ApplicationResumeAgentEvent, error) {
	event.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	query, arguments, err := sqlBuilder.
		Insert("application_resume_agent_events").
		Columns("trigger_message_id", "revision_id", "event_type", "detail", "created_at").
		Values(event.TriggerMessageID, nullableInt64(event.RevisionID), event.Type, event.Detail, event.CreatedAt).
		ToSql()
	if err != nil {
		return models.ApplicationResumeAgentEvent{}, fmt.Errorf("build create application resume agent event query: %w", err)
	}
	result, err := repository.db.ExecContext(ctx, query, arguments...)
	if err != nil {
		return models.ApplicationResumeAgentEvent{}, fmt.Errorf("create application resume agent event: %w", err)
	}
	event.ID, err = result.LastInsertId()
	if err != nil {
		return models.ApplicationResumeAgentEvent{}, fmt.Errorf("get application resume agent event ID: %w", err)
	}
	return event, nil
}

func nullableInt64(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}

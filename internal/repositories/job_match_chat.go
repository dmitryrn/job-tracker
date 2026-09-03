package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) JobMatchChatMessages(ctx context.Context, jobID int64) ([]models.JobMatchChatMessage, error) {
	query, arguments, err := sqlBuilder.
		Select("id", "job_id", "role", "content", "created_at", "COALESCE(request_id, '')").
		From("job_match_chat_messages").
		Where(squirrel.Eq{"job_id": jobID}).
		OrderBy("id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list job match chat messages query: %w", err)
	}

	rows, err := repository.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list job match chat messages: %w", err)
	}
	defer rows.Close()

	messages := make([]models.JobMatchChatMessage, 0)
	for rows.Next() {
		var message models.JobMatchChatMessage
		if err := rows.Scan(&message.ID, &message.JobID, &message.Role, &message.Content, &message.CreatedAt, &message.RequestID); err != nil {
			return nil, fmt.Errorf("scan job match chat message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job match chat messages: %w", err)
	}
	return messages, nil
}

func (repository *SQLite) JobMatchChatMessageByRequestID(ctx context.Context, jobID int64, requestID, role string) (*models.JobMatchChatMessage, error) {
	query, arguments, err := sqlBuilder.
		Select("id", "job_id", "role", "content", "created_at", "COALESCE(request_id, '')").
		From("job_match_chat_messages").
		Where(squirrel.Eq{"job_id": jobID, "request_id": requestID, "role": role}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get job match chat message by request ID query: %w", err)
	}

	var message models.JobMatchChatMessage
	err = repository.db.QueryRowContext(ctx, query, arguments...).Scan(&message.ID, &message.JobID, &message.Role, &message.Content, &message.CreatedAt, &message.RequestID)
	if err == nil {
		return &message, nil
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return nil, fmt.Errorf("get job match chat message by request ID: %w", err)
}

func (repository *SQLite) CreateJobMatchChatMessage(ctx context.Context, message models.JobMatchChatMessage) (models.JobMatchChatMessage, error) {
	query, arguments, err := sqlBuilder.
		Insert("job_match_chat_messages").
		Columns("job_id", "role", "content", "created_at", "request_id").
		Values(message.JobID, message.Role, message.Content, message.CreatedAt, nullableString(message.RequestID)).
		Suffix("ON CONFLICT DO NOTHING").
		ToSql()
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("build create job match chat message query: %w", err)
	}

	result, err := repository.db.ExecContext(ctx, query, arguments...)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("create job match chat message: %w", err)
	}
	created, err := result.RowsAffected()
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("check job match chat message creation: %w", err)
	}
	if created == 0 {
		existing, err := repository.JobMatchChatMessageByRequestID(ctx, message.JobID, message.RequestID, message.Role)
		if err != nil {
			return models.JobMatchChatMessage{}, err
		}
		if existing == nil {
			return models.JobMatchChatMessage{}, fmt.Errorf("find existing job match chat message: no message found")
		}
		return *existing, nil
	}
	message.ID, err = result.LastInsertId()
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("get job match chat message ID: %w", err)
	}
	return message, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

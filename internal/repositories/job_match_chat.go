package repositories

import (
	"context"
	"fmt"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) JobMatchChatMessages(ctx context.Context, jobID int64) ([]models.JobMatchChatMessage, error) {
	query, arguments, err := sqlBuilder.
		Select("id", "job_id", "role", "content", "created_at").
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
		if err := rows.Scan(&message.ID, &message.JobID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan job match chat message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job match chat messages: %w", err)
	}
	return messages, nil
}

func (repository *SQLite) CreateJobMatchChatMessage(ctx context.Context, message models.JobMatchChatMessage) (models.JobMatchChatMessage, error) {
	query, arguments, err := sqlBuilder.
		Insert("job_match_chat_messages").
		Columns("job_id", "role", "content", "created_at").
		Values(message.JobID, message.Role, message.Content, message.CreatedAt).
		ToSql()
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("build create job match chat message query: %w", err)
	}

	result, err := repository.db.ExecContext(ctx, query, arguments...)
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("create job match chat message: %w", err)
	}
	message.ID, err = result.LastInsertId()
	if err != nil {
		return models.JobMatchChatMessage{}, fmt.Errorf("get job match chat message ID: %w", err)
	}
	return message, nil
}

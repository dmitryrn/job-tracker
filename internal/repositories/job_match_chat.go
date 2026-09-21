package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) JobMatchChatItems(ctx context.Context, jobID, afterSequence int64) ([]models.JobMatchChatItem, error) {
	query, arguments, err := sqlBuilder.
		Select("job_id", "sequence", "item_type", "payload_json", "COALESCE(request_id, '')", "created_at").
		From("job_match_chat_items").
		Where(squirrel.Eq{"job_id": jobID}).
		Where(squirrel.Gt{"sequence": afterSequence}).
		OrderBy("sequence").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list job match chat items query: %w", err)
	}

	rows, err := repository.db.QueryContext(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list job match chat items: %w", err)
	}
	defer rows.Close()

	items := make([]models.JobMatchChatItem, 0)
	for rows.Next() {
		var item models.JobMatchChatItem
		var itemPayload []byte
		if err := rows.Scan(&item.JobID, &item.Sequence, &item.Type, &itemPayload, &item.RequestID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan job match chat item: %w", err)
		}

		item.Payload = json.RawMessage(itemPayload)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate job match chat items: %w", err)
	}

	return items, nil
}

func (repository *SQLite) JobMatchChatItemByRequestID(ctx context.Context, jobID int64, requestID, itemType string) (*models.JobMatchChatItem, error) {
	query, arguments, err := sqlBuilder.
		Select("job_id", "sequence", "item_type", "payload_json", "COALESCE(request_id, '')", "created_at").
		From("job_match_chat_items").
		Where(squirrel.Eq{"job_id": jobID, "request_id": requestID, "item_type": itemType}).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get job match chat item by request ID query: %w", err)
	}

	var item models.JobMatchChatItem
	var itemPayload []byte
	err = repository.db.QueryRowContext(ctx, query, arguments...).Scan(&item.JobID, &item.Sequence, &item.Type, &itemPayload, &item.RequestID, &item.CreatedAt)
	if err == nil {
		item.Payload = json.RawMessage(itemPayload)
		return &item, nil
	}

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	return nil, fmt.Errorf("get job match chat item by request ID: %w", err)
}

func (repository *SQLite) CreateJobMatchChatItem(ctx context.Context, item models.JobMatchChatItem) (models.JobMatchChatItem, error) {
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("begin create job match chat item: %w", err)
	}
	defer transaction.Rollback()

	sequenceQuery, sequenceArguments, err := sqlBuilder.
		Select("COALESCE(MAX(sequence), 0) + 1").
		From("job_match_chat_items").
		Where(squirrel.Eq{"job_id": item.JobID}).
		ToSql()
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("build next job match chat item sequence query: %w", err)
	}

	if err := transaction.QueryRowContext(ctx, sequenceQuery, sequenceArguments...).Scan(&item.Sequence); err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("get next job match chat item sequence: %w", err)
	}

	item.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	query, arguments, err := sqlBuilder.
		Insert("job_match_chat_items").
		Columns("job_id", "sequence", "item_type", "payload_json", "request_id", "created_at").
		Values(item.JobID, item.Sequence, item.Type, string(item.Payload), nullableString(item.RequestID), item.CreatedAt).
		ToSql()
	if err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("build create job match chat item query: %w", err)
	}

	if _, err := transaction.ExecContext(ctx, query, arguments...); err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("create job match chat item: %w", err)
	}

	if err := transaction.Commit(); err != nil {
		return models.JobMatchChatItem{}, fmt.Errorf("commit create job match chat item: %w", err)
	}

	return item, nil
}

func (repository *SQLite) DeleteJobMatchChatItemsFrom(ctx context.Context, jobID, sequence int64) (bool, error) {
	query, arguments, err := sqlBuilder.
		Delete("job_match_chat_items").
		Where(squirrel.Eq{"job_id": jobID}).
		Where(squirrel.GtOrEq{"sequence": sequence}).
		ToSql()
	if err != nil {
		return false, fmt.Errorf("build delete job match chat items query: %w", err)
	}

	result, err := repository.db.ExecContext(ctx, query, arguments...)
	if err != nil {
		return false, fmt.Errorf("delete job match chat items: %w", err)
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("check job match chat item deletion: %w", err)
	}

	return deleted > 0, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

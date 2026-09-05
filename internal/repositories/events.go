package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"nice/internal/models"
)

func (repository *SQLite) RecordEvent(ctx context.Context, event models.Event) error {
	return repository.RecordEvents(ctx, []models.Event{event})
}

func (repository *SQLite) RecordEvents(ctx context.Context, events []models.Event) error {
	if len(events) == 0 {
		return nil
	}
	transaction, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin event transaction: %w", err)
	}
	defer transaction.Rollback()

	for _, event := range events {
		data := event.Data
		if data == nil {
			data = map[string]any{}
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("encode event data: %w", err)
		}
		if event.OccurredAt == "" {
			event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
		}
		if _, err := transaction.ExecContext(ctx, `
			INSERT INTO app_events (occurred_at, provider, run_id, type, level, message, data_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			event.OccurredAt, event.Provider, event.RunID, event.Type, event.Level, event.Message, string(encoded),
		); err != nil {
			return fmt.Errorf("record event: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit event transaction: %w", err)
	}
	return nil
}

func (repository *SQLite) Events(ctx context.Context, search models.EventSearch) (models.EventPage, error) {
	query := sqlBuilder.Select("id", "occurred_at", "provider", "run_id", "type", "level", "message", "data_json").
		From("app_events").
		OrderBy("occurred_at DESC", "id DESC").
		Limit(uint64(search.Limit)).
		Offset(uint64(search.Offset))
	countQuery := sqlBuilder.Select("COUNT(*)").From("app_events")
	if search.Provider != "" {
		query = query.Where(squirrel.Eq{"provider": search.Provider})
		countQuery = countQuery.Where(squirrel.Eq{"provider": search.Provider})
	}
	if search.RunID != "" {
		query = query.Where(squirrel.Eq{"run_id": search.RunID})
		countQuery = countQuery.Where(squirrel.Eq{"run_id": search.RunID})
	}
	if search.Type != "" {
		query = query.Where(squirrel.Like{"type": "%" + search.Type + "%"})
		countQuery = countQuery.Where(squirrel.Like{"type": "%" + search.Type + "%"})
	}
	if search.Level != "" {
		query = query.Where(squirrel.Eq{"level": search.Level})
		countQuery = countQuery.Where(squirrel.Eq{"level": search.Level})
	}
	statement, args, err := query.ToSql()
	if err != nil {
		return models.EventPage{}, fmt.Errorf("build events query: %w", err)
	}
	rows, err := repository.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return models.EventPage{}, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	page := models.EventPage{Events: make([]models.Event, 0)}
	for rows.Next() {
		var event models.Event
		var data string
		if err := rows.Scan(&event.ID, &event.OccurredAt, &event.Provider, &event.RunID, &event.Type, &event.Level, &event.Message, &data); err != nil {
			return models.EventPage{}, fmt.Errorf("scan event: %w", err)
		}
		if err := json.Unmarshal([]byte(data), &event.Data); err != nil {
			return models.EventPage{}, fmt.Errorf("decode event data: %w", err)
		}
		page.Events = append(page.Events, event)
	}
	if err := rows.Err(); err != nil {
		return models.EventPage{}, fmt.Errorf("iterate events: %w", err)
	}
	countStatement, countArgs, err := countQuery.ToSql()
	if err != nil {
		return models.EventPage{}, fmt.Errorf("build event count query: %w", err)
	}
	if err := repository.db.QueryRowContext(ctx, countStatement, countArgs...).Scan(&page.Total); err != nil {
		return models.EventPage{}, fmt.Errorf("count events: %w", err)
	}
	return page, nil
}

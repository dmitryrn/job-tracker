package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/repositories"
)

type eventBatchRecorder struct {
	batches chan []models.Event
}

type blockingEventBatchRecorder struct {
	eventBatchRecorder
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type retryingEventBatchRecorder struct {
	eventBatchRecorder
	attempts int
}

func TestEventWriterFlushesAtBatchSize(t *testing.T) {
	repository := &eventBatchRecorder{batches: make(chan []models.Event, 1)}
	writer := newTestEventWriter(repository, 10, 2, time.Hour)
	writer.start(context.Background())
	t.Cleanup(func() { require.NoError(t, writer.stop(context.Background())) })

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "first"}))
	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "second"}))

	batch := receiveEventBatch(t, repository.batches)
	assert.Equal(t, []string{"first", "second"}, []string{batch[0].Type, batch[1].Type})
	assert.NotEmpty(t, batch[0].OccurredAt)
}

func TestEventWriterFlushesAfterInterval(t *testing.T) {
	repository := &eventBatchRecorder{batches: make(chan []models.Event, 1)}
	writer := newTestEventWriter(repository, 10, 10, 10*time.Millisecond)
	writer.start(context.Background())
	t.Cleanup(func() { require.NoError(t, writer.stop(context.Background())) })

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "scheduled"}))

	batch := receiveEventBatchWithin(t, repository.batches, 3*time.Second)
	require.Len(t, batch, 1)
	assert.Equal(t, "scheduled", batch[0].Type)
}

func TestEventWriterBlocksWhenQueueIsFull(t *testing.T) {
	repository := &blockingEventBatchRecorder{started: make(chan struct{}), release: make(chan struct{})}
	writer := newTestEventWriter(repository, 1, 1, time.Hour)
	writer.start(context.Background())
	t.Cleanup(func() { require.NoError(t, writer.stop(context.Background())) })

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "first"}))
	<-repository.started
	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "second"}))
	completed := make(chan error, 1)
	go func() {
		completed <- writer.RecordEvent(context.Background(), models.Event{Type: "third"})
	}()

	select {
	case err := <-completed:
		t.Fatalf("third event was not backpressured: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(repository.release)
	require.NoError(t, <-completed)
}

func TestEventWriterFlushesQueuedEventsDuringShutdown(t *testing.T) {
	repository := &eventBatchRecorder{batches: make(chan []models.Event, 1)}
	writer := newTestEventWriter(repository, 10, 10, time.Hour)
	writer.start(context.Background())

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "pending"}))
	require.NoError(t, writer.stop(context.Background()))

	batch := receiveEventBatch(t, repository.batches)
	require.Len(t, batch, 1)
	assert.Equal(t, "pending", batch[0].Type)
}

func TestEventWriterRetriesFailedBatch(t *testing.T) {
	repository := &retryingEventBatchRecorder{eventBatchRecorder: eventBatchRecorder{batches: make(chan []models.Event, 1)}}
	writer := newTestEventWriter(repository, 10, 1, time.Hour)
	writer.start(context.Background())
	t.Cleanup(func() { require.NoError(t, writer.stop(context.Background())) })

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "retry"}))

	batch := receiveEventBatchWithin(t, repository.batches, 3*time.Second)
	assert.Equal(t, "retry", batch[0].Type)
	assert.Equal(t, 2, repository.attempts)
}

func newTestEventWriter(repository repositories.EventRepository, queueSize, batchSize int, flushInterval time.Duration) *EventWriter {
	return NewEventWriter(config.Config{Events: config.EventConfig{QueueSize: queueSize, BatchSize: batchSize, FlushInterval: flushInterval}}, repository, zap.NewNop())
}

func receiveEventBatch(t *testing.T, batches <-chan []models.Event) []models.Event {
	t.Helper()
	return receiveEventBatchWithin(t, batches, time.Second)
}

func receiveEventBatchWithin(t *testing.T, batches <-chan []models.Event, timeout time.Duration) []models.Event {
	t.Helper()
	select {
	case batch := <-batches:
		return batch
	case <-time.After(timeout):
		t.Fatal("event batch was not recorded")
		return nil
	}
}

func (recorder *eventBatchRecorder) RecordEvent(context.Context, models.Event) error {
	return nil
}

func (recorder *eventBatchRecorder) RecordEvents(_ context.Context, events []models.Event) error {
	batch := append([]models.Event(nil), events...)
	recorder.batches <- batch
	return nil
}

func (*eventBatchRecorder) Events(context.Context, models.EventSearch) (models.EventPage, error) {
	return models.EventPage{}, nil
}

func (recorder *blockingEventBatchRecorder) RecordEvents(_ context.Context, _ []models.Event) error {
	recorder.once.Do(func() {
		close(recorder.started)
		<-recorder.release
	})
	return nil
}

func (recorder *retryingEventBatchRecorder) RecordEvents(_ context.Context, events []models.Event) error {
	recorder.attempts++
	if recorder.attempts == 1 {
		return errors.New("temporary event storage failure")
	}

	recorder.batches <- append([]models.Event(nil), events...)
	return nil
}

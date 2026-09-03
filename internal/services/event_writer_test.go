package services

import (
	"context"
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

func TestEventWriterFlushesAtBatchSize(t *testing.T) {
	repository := &eventBatchRecorder{batches: make(chan []models.Event, 1)}
	writer := newTestEventWriter(repository, 10, 2, time.Hour)
	writer.start()
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
	writer.start()
	t.Cleanup(func() { require.NoError(t, writer.stop(context.Background())) })

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "scheduled"}))

	batch := receiveEventBatch(t, repository.batches)
	require.Len(t, batch, 1)
	assert.Equal(t, "scheduled", batch[0].Type)
}

func TestEventWriterBlocksWhenQueueIsFull(t *testing.T) {
	repository := &blockingEventBatchRecorder{started: make(chan struct{}), release: make(chan struct{})}
	writer := newTestEventWriter(repository, 1, 1, time.Hour)
	writer.start()
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
	writer.start()

	require.NoError(t, writer.RecordEvent(context.Background(), models.Event{Type: "pending"}))
	require.NoError(t, writer.stop(context.Background()))

	batch := receiveEventBatch(t, repository.batches)
	require.Len(t, batch, 1)
	assert.Equal(t, "pending", batch[0].Type)
}

func newTestEventWriter(repository repositories.EventRepository, queueSize, batchSize int, flushInterval time.Duration) *EventWriter {
	return NewEventWriter(config.Config{Events: config.EventConfig{QueueSize: queueSize, BatchSize: batchSize, FlushInterval: flushInterval}}, repository, zap.NewNop())
}

func receiveEventBatch(t *testing.T, batches <-chan []models.Event) []models.Event {
	t.Helper()
	select {
	case batch := <-batches:
		return batch
	case <-time.After(time.Second):
		t.Fatal("event batch was not recorded")
		return nil
	}
}

type eventBatchRecorder struct {
	batches chan []models.Event
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

type blockingEventBatchRecorder struct {
	eventBatchRecorder
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (recorder *blockingEventBatchRecorder) RecordEvents(ctx context.Context, events []models.Event) error {
	recorder.once.Do(func() {
		close(recorder.started)
		<-recorder.release
	})
	return nil
}

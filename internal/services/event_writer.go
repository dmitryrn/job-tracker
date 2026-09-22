package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"

	"nice/internal/config"
	"nice/internal/models"
	"nice/internal/repositories"
)

const eventWriteRetryInterval = time.Second

var ErrEventWriterStopped = errors.New("event writer is stopped")

type EventWriter struct {
	repository    repositories.EventRepository
	logger        *zap.Logger
	batchSize     int
	flushInterval time.Duration
	queue         chan models.Event

	mutex   sync.RWMutex
	running bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewEventWriter(cfg config.Config, repository repositories.EventRepository, logger *zap.Logger) *EventWriter {
	return &EventWriter{
		repository:    repository,
		logger:        logger,
		batchSize:     cfg.Events.BatchSize,
		flushInterval: cfg.Events.FlushInterval,
		queue:         make(chan models.Event, cfg.Events.QueueSize),
	}
}

func NewEventRecorder(writer *EventWriter) repositories.EventRecorder {
	return writer
}

func (writer *EventWriter) Register(lifecycle fx.Lifecycle) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			writer.start(ctx)
			return nil
		},
		OnStop: writer.stop,
	})
}

func (writer *EventWriter) RecordEvent(_ context.Context, event models.Event) error {
	writer.mutex.RLock()
	defer writer.mutex.RUnlock()
	if !writer.running {
		return ErrEventWriterStopped
	}

	event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	writer.queue <- event
	return nil
}

func (writer *EventWriter) start(startContext context.Context) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	if writer.running {
		return
	}

	ctx, cancel := context.WithCancel(context.WithoutCancel(startContext))
	writer.cancel = cancel
	writer.done = make(chan struct{})
	writer.running = true
	writer.logger.Info("event writer started", zap.Int("queue_size", cap(writer.queue)), zap.Int("batch_size", writer.batchSize), zap.Duration("flush_interval", writer.flushInterval))
	go writer.run(ctx)
}

func (writer *EventWriter) stop(ctx context.Context) error {
	writer.mutex.Lock()
	if !writer.running {
		writer.mutex.Unlock()
		return nil
	}

	writer.running = false
	cancel := writer.cancel
	done := writer.done
	writer.mutex.Unlock()

	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		writer.logger.Error("event writer shutdown timed out", zap.Error(ctx.Err()))
		return ctx.Err()
	}
}

func (writer *EventWriter) run(ctx context.Context) {
	defer close(writer.done)
	batch := make([]models.Event, 0, writer.batchSize)
	timer := time.NewTimer(writer.flushInterval)
	stopTimer(timer)
	defer timer.Stop()

	for {
		var timerChannel <-chan time.Time
		if len(batch) > 0 {
			timerChannel = timer.C
		}

		select {
		case event := <-writer.queue:
			batch = append(batch, event)
			if len(batch) == 1 {
				timer.Reset(writer.flushInterval)
			}

			if len(batch) == writer.batchSize {
				stopTimer(timer)
				writer.flush(context.WithoutCancel(ctx), &batch)
			}
		case <-timerChannel:
			writer.flush(context.WithoutCancel(ctx), &batch)
		case <-ctx.Done():
			writer.drain(context.WithoutCancel(ctx), &batch)
			writer.logger.Info("event writer stopped")
			return
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer.Stop() {
		return
	}

	select {
	case <-timer.C:
	default:
	}
}

func (writer *EventWriter) flush(ctx context.Context, batch *[]models.Event) {
	if len(*batch) == 0 {
		return
	}

	for {
		err := writer.repository.RecordEvents(ctx, *batch)
		if err == nil {
			writer.logger.Info("event batch recorded", zap.Int("count", len(*batch)))
			*batch = (*batch)[:0]
			return
		}

		writer.logger.Error("record event batch failed", zap.Int("count", len(*batch)), zap.Error(err))
		time.Sleep(eventWriteRetryInterval)
	}
}

func (writer *EventWriter) drain(ctx context.Context, batch *[]models.Event) {
	for {
		select {
		case event := <-writer.queue:
			*batch = append(*batch, event)
		default:
			writer.flush(ctx, batch)
			return
		}
	}
}

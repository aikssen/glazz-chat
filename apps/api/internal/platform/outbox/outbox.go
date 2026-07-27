package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/logging"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

type Handler interface {
	Handle(ctx context.Context, event Event) error
}

type HandlerFunc func(context.Context, Event) error

func (function HandlerFunc) Handle(ctx context.Context, event Event) error {
	return function(ctx, event)
}

type Event struct {
	ID      string
	Type    string
	Payload json.RawMessage
}

type Runner struct {
	database    *database.Pool
	clock       clock.Clock
	logger      *slog.Logger
	workerID    string
	handlers    map[string]Handler
	batchSize   int32
	maxAttempts int32
	lockTTL     time.Duration
	pollEvery   time.Duration
}

type Options struct {
	WorkerID    string
	BatchSize   int32
	MaxAttempts int32
	LockTTL     time.Duration
	PollEvery   time.Duration
}

func New(
	pool *database.Pool,
	timeSource clock.Clock,
	logger *slog.Logger,
	handlers map[string]Handler,
	options Options,
) (*Runner, error) {
	if options.WorkerID == "" || options.BatchSize <= 0 || options.MaxAttempts <= 0 ||
		options.LockTTL <= 0 || options.PollEvery <= 0 {
		return nil, errors.New("outbox runner options must be positive and WorkerID must be set")
	}
	return &Runner{
		database:    pool,
		clock:       timeSource,
		logger:      logger,
		workerID:    options.WorkerID,
		handlers:    handlers,
		batchSize:   options.BatchSize,
		maxAttempts: options.MaxAttempts,
		lockTTL:     options.LockTTL,
		pollEvery:   options.PollEvery,
	}, nil
}

func (runner *Runner) Run(ctx context.Context) error {
	runner.logger.InfoContext(ctx, "outbox runner started",
		"worker_id", runner.workerID,
		"poll_interval_ms", runner.pollEvery.Milliseconds(),
	)
	ticker := time.NewTicker(runner.pollEvery)
	defer ticker.Stop()
	for {
		if _, err := runner.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			runner.logger.ErrorContext(ctx, "outbox cycle failed", "error_class", fmt.Sprintf("%T", err))
		}
		select {
		case <-ctx.Done():
			runner.logger.InfoContext(ctx, "outbox runner stopped", "worker_id", runner.workerID)
			return nil
		case <-ticker.C:
		}
	}
}

func (runner *Runner) RunOnce(ctx context.Context) (int, error) {
	runner.logger.DebugContext(ctx, "outbox claim started", "worker_id", runner.workerID)
	now := runner.clock.Now()
	events, err := runner.database.Queries().ClaimOutboxEvents(ctx, store.ClaimOutboxEventsParams{
		NowAt:             timestamp(now),
		LockExpiredBefore: timestamp(now.Add(-runner.lockTTL)),
		BatchSize:         runner.batchSize,
		WorkerID:          &runner.workerID,
	})
	if err != nil {
		return 0, fmt.Errorf("claim outbox events: %w", err)
	}
	runner.logger.DebugContext(ctx, "outbox claim completed",
		"worker_id", runner.workerID,
		"event_count", len(events),
	)

	var wait sync.WaitGroup
	errorsChannel := make(chan error, len(events))
	for _, record := range events {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := runner.process(ctx, record); err != nil {
				errorsChannel <- err
			}
		}()
	}
	wait.Wait()
	close(errorsChannel)

	var joined error
	for err := range errorsChannel {
		joined = errors.Join(joined, err)
	}
	return len(events), joined
}

func (runner *Runner) process(ctx context.Context, record store.OutboxEvent) error {
	ctx = logging.WithCorrelationID(ctx, record.ID.String())
	logger := logging.Context(runner.logger, ctx).With(
		"event_id", record.ID,
		"event_type", record.EventType,
		"attempt", record.Attempts,
	)
	logger.DebugContext(ctx, "outbox event processing started")
	acquired, err := runner.database.WithAdvisoryLock(
		ctx,
		"outbox:"+record.ID.String(),
		func() error { return runner.processLocked(ctx, record) },
	)
	if err != nil {
		logger.ErrorContext(ctx, "outbox event processing failed",
			"error_class", fmt.Sprintf("%T", err),
		)
		return err
	}
	if !acquired {
		logger.DebugContext(ctx, "outbox event skipped because lock is held")
		return nil
	}
	logger.DebugContext(ctx, "outbox event processing finished")
	return nil
}

func (runner *Runner) processLocked(ctx context.Context, record store.OutboxEvent) error {
	handler, ok := runner.handlers[record.EventType]
	if !ok {
		return runner.fail(ctx, record, errors.New("outbox handler is not registered"))
	}
	handlerName := record.EventType + ".v1"
	handled, err := runner.database.Queries().HasOutboxReceipt(ctx, store.HasOutboxReceiptParams{
		EventID:     record.ID,
		HandlerName: handlerName,
	})
	if err != nil {
		return fmt.Errorf("read outbox receipt: %w", err)
	}
	if !handled {
		logging.Context(runner.logger, ctx).DebugContext(ctx, "outbox handler started",
			"event_id", record.ID,
			"event_type", record.EventType,
		)
		event := Event{ID: record.ID.String(), Type: record.EventType, Payload: record.Payload}
		if err := handler.Handle(ctx, event); err != nil {
			return runner.fail(ctx, record, err)
		}
	}

	now := runner.clock.Now()
	err = runner.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		if err := queries.RecordOutboxReceipt(ctx, store.RecordOutboxReceiptParams{
			EventID:     record.ID,
			HandlerName: handlerName,
		}); err != nil {
			return fmt.Errorf("record outbox receipt: %w", err)
		}
		updated, err := queries.CompleteOutboxEvent(ctx, store.CompleteOutboxEventParams{
			ID:          record.ID,
			LockedBy:    &runner.workerID,
			ProcessedAt: timestamp(now),
		})
		if err != nil {
			return fmt.Errorf("complete outbox event: %w", err)
		}
		if updated != 1 {
			return errors.New("outbox event lock was lost before completion")
		}
		return nil
	})
	if err == nil {
		logging.Context(runner.logger, ctx).InfoContext(ctx, "outbox event completed",
			"event_id", record.ID,
			"event_type", record.EventType,
		)
	}
	return err
}

func (runner *Runner) fail(ctx context.Context, record store.OutboxEvent, handlerError error) error {
	now := runner.clock.Now()
	errorClass := fmt.Sprintf("%T", handlerError)
	if record.Attempts >= runner.maxAttempts {
		_, err := runner.database.Queries().DeadLetterOutboxEvent(ctx, store.DeadLetterOutboxEventParams{
			ID:             record.ID,
			LockedBy:       &runner.workerID,
			DeadLetteredAt: timestamp(now),
			LastErrorClass: &errorClass,
		})
		if err != nil {
			return fmt.Errorf("dead-letter outbox event: %w", err)
		}
		logging.Context(runner.logger, ctx).ErrorContext(ctx, "outbox event dead-lettered",
			"event_id", record.ID,
			"event_type", record.EventType,
			"attempt", record.Attempts,
			"error_class", errorClass,
		)
		return nil
	}
	delay := time.Second * time.Duration(math.Pow(2, float64(record.Attempts-1)))
	if delay > time.Hour {
		delay = time.Hour
	}
	_, err := runner.database.Queries().RetryOutboxEvent(ctx, store.RetryOutboxEventParams{
		ID:             record.ID,
		LockedBy:       &runner.workerID,
		AvailableAt:    timestamp(now.Add(delay)),
		LastErrorClass: &errorClass,
	})
	if err != nil {
		return fmt.Errorf("schedule outbox retry: %w", err)
	}
	logging.Context(runner.logger, ctx).WarnContext(ctx, "outbox event scheduled for retry",
		"event_id", record.ID,
		"event_type", record.EventType,
		"attempt", record.Attempts,
		"retry_delay_ms", delay.Milliseconds(),
		"error_class", errorClass,
	)
	return nil
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

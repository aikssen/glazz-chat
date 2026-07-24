package privacy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrNotFound = errors.New("account deletion not found")
	ErrConflict = errors.New("account deletion state conflict")
)

type Deletion struct {
	ID          uuid.UUID  `json:"id"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requestedAt"`
	DueAt       time.Time  `json:"dueAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type Service struct {
	database *database.Pool
	ids      ids.Source
	clock    clock.Clock
}

func New(pool *database.Pool, idSource ids.Source, timeSource clock.Clock) *Service {
	return &Service{database: pool, ids: idSource, clock: timeSource}
}

func (service *Service) Request(ctx context.Context, userID uuid.UUID) (Deletion, error) {
	if userID == uuid.Nil {
		return Deletion{}, ErrNotFound
	}
	now := service.clock.Now().UTC()
	var job store.AccountDeletionJob
	err := service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		existing, err := queries.GetAccountDeletionJobByUser(ctx, &userID)
		if err == nil {
			job = existing
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		user, err := queries.GetAdminUser(ctx, userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if user.Status != "active" {
			return ErrConflict
		}
		changed, err := queries.MarkUserDeletionPending(ctx, store.MarkUserDeletionPendingParams{
			NowAt: timestamp(now), ID: userID,
		})
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrConflict
		}
		if err := queries.RevokeAllUserSessions(ctx, store.RevokeAllUserSessionsParams{
			NowAt: timestamp(now), UserID: userID,
		}); err != nil {
			return err
		}
		id, err := service.ids.New()
		if err != nil {
			return err
		}
		job, err = queries.CreateAccountDeletionJob(ctx, store.CreateAccountDeletionJobParams{
			ID: id, UserID: &userID, RequestedAt: timestamp(now),
			DueAt: timestamp(now.Add(24 * time.Hour)),
		})
		return err
	})
	if err != nil {
		return Deletion{}, fmt.Errorf("request account deletion: %w", err)
	}
	return mapDeletion(job), nil
}

func (service *Service) Get(ctx context.Context, userID uuid.UUID) (Deletion, error) {
	job, err := service.database.Queries().GetAccountDeletionJobByUser(ctx, &userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Deletion{}, ErrNotFound
	}
	if err != nil {
		return Deletion{}, fmt.Errorf("get account deletion: %w", err)
	}
	return mapDeletion(job), nil
}

func (service *Service) PurgeDue(ctx context.Context, grace time.Duration, batchSize int32) (int, error) {
	if grace < 0 || batchSize <= 0 || batchSize > 100 {
		return 0, errors.New("purge options are invalid")
	}
	now := service.clock.Now().UTC()
	jobs, err := service.database.Queries().ClaimAccountDeletionJobs(
		ctx,
		store.ClaimAccountDeletionJobsParams{
			NowAt: timestamp(now), RequestedBefore: timestamp(now.Add(-grace)), BatchSize: batchSize,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("claim deletion jobs: %w", err)
	}
	completed := 0
	var joined error
	for _, job := range jobs {
		if job.UserID == nil {
			joined = errors.Join(joined, service.fail(ctx, job.ID, errors.New("job user is missing")))
			continue
		}
		err := service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
			deleted, err := queries.DeleteUserForPurge(ctx, *job.UserID)
			if err != nil {
				return err
			}
			if deleted != 1 {
				return ErrConflict
			}
			updated, err := queries.CompleteAccountDeletionJob(ctx, store.CompleteAccountDeletionJobParams{
				NowAt: timestamp(now), ID: job.ID,
			})
			if err != nil {
				return err
			}
			if updated != 1 {
				return ErrConflict
			}
			return nil
		})
		if err != nil {
			joined = errors.Join(joined, service.fail(ctx, job.ID, err))
			continue
		}
		completed++
	}
	return completed, joined
}

func (service *Service) CleanupGuests(ctx context.Context) (int64, error) {
	count, err := service.database.Queries().DeleteExpiredGuestSessions(
		ctx,
		timestamp(service.clock.Now()),
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired guests: %w", err)
	}
	return count, nil
}

func (service *Service) fail(ctx context.Context, jobID uuid.UUID, cause error) error {
	class := fmt.Sprintf("%T", cause)
	_, updateErr := service.database.Queries().FailAccountDeletionJob(
		ctx,
		store.FailAccountDeletionJobParams{LastErrorClass: &class, ID: jobID},
	)
	return errors.Join(cause, updateErr)
}

func mapDeletion(record store.AccountDeletionJob) Deletion {
	result := Deletion{
		ID: record.ID, Status: record.Status,
		RequestedAt: record.RequestedAt.Time.UTC(), DueAt: record.DueAt.Time.UTC(),
	}
	if record.CompletedAt.Valid {
		completed := record.CompletedAt.Time.UTC()
		result.CompletedAt = &completed
	}
	return result
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

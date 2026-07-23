package quota

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/aikssen/glazz-chat/apps/api/internal/platform/clock"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/database"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/ids"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/redisx"
	"github.com/aikssen/glazz-chat/apps/api/internal/platform/store"
)

var (
	ErrExceeded = errors.New("quota exceeded")
	ErrBusy     = errors.New("actor already has an active generation")
	ErrInvalid  = errors.New("invalid quota request")
)

type ActorType string

const (
	Guest ActorType = "guest"
	User  ActorType = "user"
)

type Actor struct {
	Type ActorType
	ID   uuid.UUID
	IP   netip.Addr
}

type Policy struct {
	GuestMessageLimit      int64
	GuestOutputTokenLimit  int64
	UserDailyMessageLimit  int64
	UserDailyOutputLimit   int64
	GlobalDailyMessageCap  int64
	GlobalDailyOutputCap   int64
	ActorRequestsPerMinute int64
	IPRequestsPerMinute    int64
	GenerationLeaseTTL     time.Duration
}

func DefaultPolicy() Policy {
	return Policy{
		GuestMessageLimit: 4, GuestOutputTokenLimit: 2000,
		UserDailyMessageLimit: 50, UserDailyOutputLimit: 50000,
		GlobalDailyMessageCap: 10000, GlobalDailyOutputCap: 10000000,
		ActorRequestsPerMinute: 10, IPRequestsPerMinute: 30,
		GenerationLeaseTTL: 5 * time.Minute,
	}
}

type Limiter interface {
	AddRateUsage(context.Context, string, int64, int64, time.Duration) (redisx.RateLimitResult, error)
	AcquireLease(context.Context, string, string, string, time.Duration) (bool, error)
	ReleaseLease(context.Context, string, string, string) (bool, error)
}

type Reservation struct {
	ID        uuid.UUID
	Actor     Actor
	Owner     string
	UsageDate time.Time
	Reserved  int32
	ResetAt   time.Time
}

type Service struct {
	database *database.Pool
	limiter  Limiter
	ids      ids.Source
	clock    clock.Clock
	policy   Policy
	ipKey    []byte
}

func New(
	pool *database.Pool,
	limiter Limiter,
	idSource ids.Source,
	timeSource clock.Clock,
	policy Policy,
	ipKey []byte,
) *Service {
	return &Service{
		database: pool, limiter: limiter, ids: idSource, clock: timeSource, policy: policy, ipKey: ipKey,
	}
}

func (service *Service) Reserve(
	ctx context.Context,
	actor Actor,
	maxOutputTokens int32,
) (Reservation, error) {
	if (actor.Type != Guest && actor.Type != User) || actor.ID == uuid.Nil ||
		maxOutputTokens <= 0 || len(service.ipKey) < 32 {
		return Reservation{}, ErrInvalid
	}
	now := service.clock.Now().UTC()
	if err := service.applyRateLimits(ctx, actor, now); err != nil {
		return Reservation{}, err
	}
	reservationID, err := service.ids.New()
	if err != nil {
		return Reservation{}, err
	}
	owner, err := ids.SecureToken(24)
	if err != nil {
		return Reservation{}, err
	}
	acquired, err := service.limiter.AcquireLease(
		ctx, "generation", actor.ID.String(), owner, service.policy.GenerationLeaseTTL,
	)
	if err != nil {
		return Reservation{}, fmt.Errorf("acquire generation lease: %w", err)
	}
	if !acquired {
		return Reservation{}, ErrBusy
	}

	usageDate := midnightUTC(now)
	release := true
	defer func() {
		if release {
			_, _ = service.limiter.ReleaseLease(
				context.WithoutCancel(ctx), "generation", actor.ID.String(), owner,
			)
		}
	}()

	err = service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		if actor.Type == Guest {
			if _, err := queries.IncrementGuestAllowance(ctx, store.IncrementGuestAllowanceParams{
				OutputTokens: maxOutputTokens,
				NowAt:        timestamp(now),
				GuestID:      actor.ID,
			}); errors.Is(err, pgx.ErrNoRows) {
				return ErrExceeded
			} else if err != nil {
				return fmt.Errorf("reserve guest allowance: %w", err)
			}
		} else {
			if _, err := queries.AddDailyUsage(ctx, dailyUsageParams(
				User, actor.ID, usageDate, maxOutputTokens,
				service.policy.UserDailyMessageLimit, service.policy.UserDailyOutputLimit, now,
			)); errors.Is(err, pgx.ErrNoRows) {
				return ErrExceeded
			} else if err != nil {
				return fmt.Errorf("reserve user allowance: %w", err)
			}
		}
		if _, err := queries.AddDailyUsage(ctx, dailyUsageParams(
			"global", uuid.Nil, usageDate, maxOutputTokens,
			service.policy.GlobalDailyMessageCap, service.policy.GlobalDailyOutputCap, now,
		)); errors.Is(err, pgx.ErrNoRows) {
			return ErrExceeded
		} else if err != nil {
			return fmt.Errorf("reserve global allowance: %w", err)
		}
		return queries.CreateQuotaReservation(ctx, store.CreateQuotaReservationParams{
			ID:                   reservationID,
			ActorType:            string(actor.Type),
			ActorID:              actor.ID,
			UsageDate:            date(usageDate),
			ReservedOutputTokens: maxOutputTokens,
		})
	})
	if err != nil {
		return Reservation{}, err
	}
	release = false
	return Reservation{
		ID: reservationID, Actor: actor, Owner: owner, UsageDate: usageDate,
		Reserved: maxOutputTokens, ResetAt: usageDate.Add(24 * time.Hour),
	}, nil
}

func (service *Service) Settle(
	ctx context.Context,
	reservation Reservation,
	actualOutputTokens int32,
) error {
	defer func() {
		_, _ = service.limiter.ReleaseLease(
			context.WithoutCancel(ctx), "generation", reservation.Actor.ID.String(), reservation.Owner,
		)
	}()
	if actualOutputTokens < 0 || actualOutputTokens > reservation.Reserved {
		return ErrInvalid
	}
	now := service.clock.Now().UTC()
	return service.database.WithinTransaction(ctx, pgx.TxOptions{}, func(queries *store.Queries) error {
		record, err := queries.LockQuotaReservation(ctx, reservation.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrInvalid
		}
		if err != nil {
			return fmt.Errorf("lock quota reservation: %w", err)
		}
		if record.Status != "reserved" {
			return nil
		}
		if record.ActorType != string(reservation.Actor.Type) ||
			record.ActorID != reservation.Actor.ID ||
			record.ReservedOutputTokens != reservation.Reserved {
			return ErrInvalid
		}
		refund := int64(record.ReservedOutputTokens - actualOutputTokens)
		if refund > 0 {
			if reservation.Actor.Type == Guest {
				if err := queries.AdjustGuestOutputUsage(ctx, store.AdjustGuestOutputUsageParams{
					DeltaTokens: -int32(refund), NowAt: timestamp(now), GuestID: reservation.Actor.ID,
				}); err != nil {
					return fmt.Errorf("refund guest allowance: %w", err)
				}
			} else if err := queries.AdjustDailyOutputUsage(ctx, store.AdjustDailyOutputUsageParams{
				DeltaTokens: -refund, NowAt: timestamp(now), ActorType: string(User),
				ActorID: reservation.Actor.ID, UsageDate: record.UsageDate,
			}); err != nil {
				return fmt.Errorf("refund user allowance: %w", err)
			}
			if err := queries.AdjustDailyOutputUsage(ctx, store.AdjustDailyOutputUsageParams{
				DeltaTokens: -refund, NowAt: timestamp(now), ActorType: "global",
				ActorID: uuid.Nil, UsageDate: record.UsageDate,
			}); err != nil {
				return fmt.Errorf("refund global allowance: %w", err)
			}
		}
		status := "committed"
		if actualOutputTokens == 0 {
			status = "refunded"
		}
		updated, err := queries.SettleQuotaReservation(ctx, store.SettleQuotaReservationParams{
			ID: reservation.ID, ActualOutputTokens: &actualOutputTokens,
			Status: status, SettledAt: timestamp(now),
		})
		if err != nil {
			return fmt.Errorf("settle quota reservation: %w", err)
		}
		if updated != 1 {
			return ErrInvalid
		}
		return nil
	})
}

func (service *Service) applyRateLimits(ctx context.Context, actor Actor, now time.Time) error {
	window := time.Minute
	bucket := now.Truncate(window).Format(time.RFC3339) + ":" + actor.ID.String()
	result, err := service.limiter.AddRateUsage(
		ctx, "actor:"+bucket, 1, service.policy.ActorRequestsPerMinute, window,
	)
	if err != nil {
		return fmt.Errorf("apply actor rate limit: %w", err)
	}
	if !result.Allowed {
		return ErrExceeded
	}
	if actor.IP.IsValid() {
		ipBucket := now.Truncate(window).Format(time.RFC3339) + ":" + HashIP(service.ipKey, actor.IP)
		result, err = service.limiter.AddRateUsage(
			ctx, "ip:"+ipBucket, 1, service.policy.IPRequestsPerMinute, window,
		)
		if err != nil {
			return fmt.Errorf("apply IP rate limit: %w", err)
		}
		if !result.Allowed {
			return ErrExceeded
		}
	}
	return nil
}

func HashIP(key []byte, address netip.Addr) string {
	return hex.EncodeToString(HashIPBytes(key, address))
}

func HashIPBytes(key []byte, address netip.Addr) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(address.Unmap().String()))
	return mac.Sum(nil)
}

func midnightUTC(value time.Time) time.Time {
	year, month, day := value.UTC().Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dailyUsageParams(
	actorType ActorType,
	actorID uuid.UUID,
	usageDate time.Time,
	outputTokens int32,
	messageLimit, outputLimit int64,
	now time.Time,
) store.AddDailyUsageParams {
	return store.AddDailyUsageParams{
		ActorType: string(actorType), ActorID: actorID, UsageDate: date(usageDate),
		OutputTokens: int64(outputTokens), NowAt: timestamp(now),
		MessageLimit: int32(messageLimit), OutputTokenLimit: outputLimit,
	}
}

func timestamp(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func date(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value, Valid: true}
}
